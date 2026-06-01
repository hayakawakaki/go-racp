package infra

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"slices"
	"time"
)

const (
	nowpaymentsLiveBaseURL    = "https://api.nowpayments.io"
	nowpaymentsSandboxBaseURL = "https://api-sandbox.nowpayments.io"
)

const (
	nowpaymentsHTTPTimeout      = 30 * time.Second
	nowpaymentsMaxResponseBytes = 1 << 20
)

type NowPaymentsClient struct {
	httpClient *http.Client
	apiKey     string
	ipnSecret  string
	baseURL    string
}

func NewNowPaymentsClient(apiKey, ipnSecret string, live bool) *NowPaymentsClient {
	baseURL := nowpaymentsSandboxBaseURL
	if live {
		baseURL = nowpaymentsLiveBaseURL
	}

	return &NowPaymentsClient{
		httpClient: &http.Client{Timeout: nowpaymentsHTTPTimeout},
		apiKey:     apiKey,
		ipnSecret:  ipnSecret,
		baseURL:    baseURL,
	}
}

type CreateInvoiceParams struct {
	PriceCurrency    string
	OrderID          string
	OrderDescription string
	SuccessURL       string
	CancelURL        string
	IPNCallbackURL   string
	PriceAmount      int64
}

type InvoiceResult struct {
	InvoiceID  string
	InvoiceURL string
}

func (c *NowPaymentsClient) CreateInvoice(ctx context.Context, params CreateInvoiceParams) (InvoiceResult, error) {
	body := map[string]any{
		"price_amount":      params.PriceAmount,
		"price_currency":    params.PriceCurrency,
		"order_id":          params.OrderID,
		"order_description": params.OrderDescription,
		"success_url":       params.SuccessURL,
		"cancel_url":        params.CancelURL,
		"ipn_callback_url":  params.IPNCallbackURL,
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		return InvoiceResult{}, fmt.Errorf("billing.nowpayments.CreateInvoice: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/invoice", bytes.NewReader(encoded))
	if err != nil {
		return InvoiceResult{}, fmt.Errorf("billing.nowpayments.CreateInvoice: %w", err)
	}

	request.Header.Set("x-api-key", c.apiKey)
	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return InvoiceResult{}, fmt.Errorf("billing.nowpayments.CreateInvoice: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	limited := io.LimitReader(response.Body, nowpaymentsMaxResponseBytes)

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(limited)
		return InvoiceResult{}, fmt.Errorf("billing.nowpayments.CreateInvoice: api status %d: %s", response.StatusCode, body)
	}

	var decoded struct {
		ID         string `json:"id"`
		InvoiceURL string `json:"invoice_url"`
	}
	if err := json.NewDecoder(limited).Decode(&decoded); err != nil {
		return InvoiceResult{}, fmt.Errorf("billing.nowpayments.CreateInvoice: %w", err)
	}

	return InvoiceResult{InvoiceID: decoded.ID, InvoiceURL: decoded.InvoiceURL}, nil
}

func (c *NowPaymentsClient) VerifyIPN(signature string, body []byte) (bool, error) {
	if c.ipnSecret == "" {
		return false, nil
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return false, fmt.Errorf("billing.nowpayments.VerifyIPN: %w", err)
	}

	var buffer bytes.Buffer
	buffer.WriteByte('{')
	for index, key := range slices.Sorted(maps.Keys(fields)) {
		if index > 0 {
			buffer.WriteByte(',')
		}

		encodedKey, err := json.Marshal(key)
		if err != nil {
			return false, fmt.Errorf("billing.nowpayments.VerifyIPN: %w", err)
		}

		buffer.Write(encodedKey)
		buffer.WriteByte(':')
		buffer.Write(fields[key])
	}
	buffer.WriteByte('}')

	mac := hmac.New(sha512.New, []byte(c.ipnSecret))
	mac.Write(buffer.Bytes())
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(signature)), nil
}
