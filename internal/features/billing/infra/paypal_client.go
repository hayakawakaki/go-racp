package infra

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	paypalLiveBaseURL    = "https://api-m.paypal.com"
	paypalSandboxBaseURL = "https://api-m.sandbox.paypal.com"
)

const (
	paypalTokenSafetyBuffer = 60 * time.Second
	paypalHTTPTimeout       = 30 * time.Second
	paypalMaxResponseBytes  = 1 << 20
)

var ErrPaypalOrderAlreadyCaptured = errors.New("billing: paypal order already captured")

type PaypalClient struct {
	httpClient  *http.Client
	clientID    string
	secret      string
	baseURL     string
	tokenExpiry time.Time
	accessToken string
	mu          sync.Mutex
}

func NewPaypalClient(clientID, secret string, live bool) *PaypalClient {
	baseURL := paypalSandboxBaseURL
	if live {
		baseURL = paypalLiveBaseURL
	}

	return &PaypalClient{
		httpClient: &http.Client{Timeout: paypalHTTPTimeout},
		clientID:   clientID,
		secret:     secret,
		baseURL:    baseURL,
	}
}

type CreateOrderParams struct {
	ReferenceID  string
	Description  string
	CurrencyCode string
	Value        string
	ReturnURL    string
	CancelURL    string
}

type OrderResult struct {
	OrderID     string
	ApprovalURL string
}

type CaptureResult struct {
	OrderID   string
	CaptureID string
	Status    string
}

type OrderDetails struct {
	OrderID     string
	Status      string
	ReferenceID string
	CaptureID   string
}

type WebhookSignatureParams struct {
	AuthAlgo         string
	CertURL          string
	TransmissionID   string
	TransmissionSig  string
	TransmissionTime string
	WebhookID        string
	Event            json.RawMessage
}

type paypalError struct {
	issue      string
	raw        string
	statusCode int
}

func (e *paypalError) Error() string {
	return fmt.Sprintf("paypal api status %d: %s", e.statusCode, e.raw)
}

func newPaypalError(statusCode int, body []byte) *paypalError {
	apiErr := &paypalError{statusCode: statusCode, raw: string(body)}

	var decoded struct {
		Details []struct {
			Issue string `json:"issue"`
		} `json:"details"`
	}
	if err := json.Unmarshal(body, &decoded); err == nil && len(decoded.Details) > 0 {
		apiErr.issue = decoded.Details[0].Issue
	}

	return apiErr
}

func (c *PaypalClient) token(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.accessToken != "" && time.Now().Before(c.tokenExpiry.Add(-paypalTokenSafetyBuffer)) {
		return c.accessToken, nil
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/oauth2/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("billing.paypal.token: %w", err)
	}

	credentials := base64.StdEncoding.EncodeToString([]byte(c.clientID + ":" + c.secret))
	request.Header.Set("Authorization", "Basic "+credentials)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("billing.paypal.token: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	limited := io.LimitReader(response.Body, paypalMaxResponseBytes)

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(limited)
		return "", fmt.Errorf("billing.paypal.token: %w", newPaypalError(response.StatusCode, body))
	}

	var decoded struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(limited).Decode(&decoded); err != nil {
		return "", fmt.Errorf("billing.paypal.token: %w", err)
	}

	c.accessToken = decoded.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(decoded.ExpiresIn) * time.Second)

	return c.accessToken, nil
}

func (c *PaypalClient) newJSONRequest(ctx context.Context, method, path, token string, body any, headers map[string]string) (*http.Request, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("billing.paypal.newJSONRequest: %w", err)
		}

		reader = bytes.NewReader(encoded)
	}

	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return nil, fmt.Errorf("billing.paypal.newJSONRequest: %w", err)
	}

	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		request.Header.Set(key, value)
	}

	return request, nil
}

func (c *PaypalClient) doJSON(ctx context.Context, method, path string, body any, headers map[string]string, result any) (int, error) {
	token, err := c.token(ctx)
	if err != nil {
		return 0, err
	}

	request, err := c.newJSONRequest(ctx, method, path, token, body, headers)
	if err != nil {
		return 0, err
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return 0, fmt.Errorf("billing.paypal.doJSON: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	limited := io.LimitReader(response.Body, paypalMaxResponseBytes)

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		raw, _ := io.ReadAll(limited)
		return response.StatusCode, fmt.Errorf("billing.paypal.doJSON: %w", newPaypalError(response.StatusCode, raw))
	}

	if result != nil {
		if err := json.NewDecoder(limited).Decode(result); err != nil {
			return response.StatusCode, fmt.Errorf("billing.paypal.doJSON: %w", err)
		}
	}

	return response.StatusCode, nil
}

func (c *PaypalClient) CreateOrder(ctx context.Context, params CreateOrderParams) (OrderResult, error) {
	body := map[string]any{
		"intent": "CAPTURE",
		"purchase_units": []map[string]any{
			{
				"reference_id": params.ReferenceID,
				"custom_id":    params.ReferenceID,
				"description":  params.Description,
				"amount": map[string]string{
					"currency_code": params.CurrencyCode,
					"value":         params.Value,
				},
			},
		},
		"application_context": map[string]string{
			"return_url":          params.ReturnURL,
			"cancel_url":          params.CancelURL,
			"user_action":         "PAY_NOW",
			"shipping_preference": "NO_SHIPPING",
		},
	}

	var decoded struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Links  []struct {
			Href string `json:"href"`
			Rel  string `json:"rel"`
		} `json:"links"`
	}

	_, err := c.doJSON(ctx, http.MethodPost, "/v2/checkout/orders", body, nil, &decoded)
	if err != nil {
		return OrderResult{}, fmt.Errorf("billing.paypal.CreateOrder: %w", err)
	}

	approvalURL := paypalApprovalURL(decoded.Links)
	if approvalURL == "" {
		return OrderResult{}, errors.New("billing.paypal.CreateOrder: missing approval link")
	}

	return OrderResult{OrderID: decoded.ID, ApprovalURL: approvalURL}, nil
}

func paypalApprovalURL(links []struct {
	Href string `json:"href"`
	Rel  string `json:"rel"`
}) string {
	var fallback string
	for _, link := range links {
		switch link.Rel {
		case "approve":
			return link.Href
		case "payer-action":
			fallback = link.Href
		}
	}

	return fallback
}

func (c *PaypalClient) CaptureOrder(ctx context.Context, orderID string) (CaptureResult, error) {
	headers := map[string]string{"PayPal-Request-Id": orderID}

	var decoded struct {
		ID            string `json:"id"`
		Status        string `json:"status"`
		PurchaseUnits []struct {
			Payments struct {
				Captures []struct {
					ID     string `json:"id"`
					Status string `json:"status"`
				} `json:"captures"`
			} `json:"payments"`
		} `json:"purchase_units"`
	}

	status, err := c.doJSON(ctx, http.MethodPost, "/v2/checkout/orders/"+orderID+"/capture", struct{}{}, headers, &decoded)
	if err != nil {
		var apiErr *paypalError
		if status == http.StatusUnprocessableEntity && errors.As(err, &apiErr) && apiErr.issue == "ORDER_ALREADY_CAPTURED" {
			return CaptureResult{}, fmt.Errorf("billing.paypal.CaptureOrder: %w", ErrPaypalOrderAlreadyCaptured)
		}

		return CaptureResult{}, fmt.Errorf("billing.paypal.CaptureOrder: %w", err)
	}

	if len(decoded.PurchaseUnits) == 0 || len(decoded.PurchaseUnits[0].Payments.Captures) == 0 {
		return CaptureResult{}, errors.New("billing.paypal.CaptureOrder: missing capture id")
	}

	capture := decoded.PurchaseUnits[0].Payments.Captures[0]
	if capture.ID == "" {
		return CaptureResult{}, errors.New("billing.paypal.CaptureOrder: missing capture id")
	}

	return CaptureResult{OrderID: decoded.ID, CaptureID: capture.ID, Status: capture.Status}, nil
}

func (c *PaypalClient) GetOrder(ctx context.Context, orderID string) (OrderDetails, error) {
	var decoded struct {
		ID            string `json:"id"`
		Status        string `json:"status"`
		PurchaseUnits []struct {
			ReferenceID string `json:"reference_id"`
			CustomID    string `json:"custom_id"`
			Payments    struct {
				Captures []struct {
					ID string `json:"id"`
				} `json:"captures"`
			} `json:"payments"`
		} `json:"purchase_units"`
	}

	_, err := c.doJSON(ctx, http.MethodGet, "/v2/checkout/orders/"+orderID, nil, nil, &decoded)
	if err != nil {
		return OrderDetails{}, fmt.Errorf("billing.paypal.GetOrder: %w", err)
	}

	details := OrderDetails{OrderID: decoded.ID, Status: decoded.Status}
	if len(decoded.PurchaseUnits) > 0 {
		unit := decoded.PurchaseUnits[0]

		details.ReferenceID = unit.CustomID
		if details.ReferenceID == "" {
			details.ReferenceID = unit.ReferenceID
		}

		if len(unit.Payments.Captures) > 0 {
			details.CaptureID = unit.Payments.Captures[0].ID
		}
	}

	return details, nil
}

func (c *PaypalClient) VerifyWebhook(ctx context.Context, params WebhookSignatureParams) (bool, error) {
	body := struct {
		AuthAlgo         string          `json:"auth_algo"`
		CertURL          string          `json:"cert_url"`
		TransmissionID   string          `json:"transmission_id"`
		TransmissionSig  string          `json:"transmission_sig"`
		TransmissionTime string          `json:"transmission_time"`
		WebhookID        string          `json:"webhook_id"`
		WebhookEvent     json.RawMessage `json:"webhook_event"`
	}{
		AuthAlgo:         params.AuthAlgo,
		CertURL:          params.CertURL,
		TransmissionID:   params.TransmissionID,
		TransmissionSig:  params.TransmissionSig,
		TransmissionTime: params.TransmissionTime,
		WebhookID:        params.WebhookID,
		WebhookEvent:     params.Event,
	}

	var decoded struct {
		VerificationStatus string `json:"verification_status"`
	}

	_, err := c.doJSON(ctx, http.MethodPost, "/v1/notifications/verify-webhook-signature", body, nil, &decoded)
	if err != nil {
		return false, fmt.Errorf("billing.paypal.VerifyWebhook: %w", err)
	}

	return decoded.VerificationStatus == "SUCCESS", nil
}
