package infra

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type paypalMockHandler struct {
	tokenBody            string
	createBody           string
	captureBody          string
	getBody              string
	verifyBody           string
	captureRequestHeader string
	createRequestBody    string
	verifyRequestBody    string
	tokenStatus          int
	createStatus         int
	captureStatus        int
	getStatus            int
	verifyStatus         int
	tokenCount           int
	createCount          int
	captureCount         int
	getCount             int
	verifyCount          int
	mu                   sync.Mutex
}

func (h *paypalMockHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()

	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/v1/oauth2/token":
		h.tokenCount++

		h.write(w, h.tokenStatus, h.tokenBody, `{"access_token":"test-token","expires_in":3600}`, http.StatusOK)
	case r.Method == http.MethodPost && r.URL.Path == "/v2/checkout/orders":
		h.createCount++

		raw, _ := io.ReadAll(r.Body)
		h.createRequestBody = string(raw)

		h.write(w, h.createStatus, h.createBody, `{"id":"ORDER1","status":"CREATED","links":[{"rel":"approve","href":"https://www.sandbox.paypal.com/checkoutnow?token=ORDER1"}]}`, http.StatusCreated)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v2/checkout/orders/") && strings.HasSuffix(r.URL.Path, "/capture"):
		h.captureCount++
		h.captureRequestHeader = r.Header.Get("PayPal-Request-Id")

		h.write(w, h.captureStatus, h.captureBody, `{"id":"ORDER1","status":"COMPLETED","purchase_units":[{"custom_id":"42","payments":{"captures":[{"id":"CAP1","status":"COMPLETED"}]}}]}`, http.StatusCreated)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v2/checkout/orders/"):
		h.getCount++

		h.write(w, h.getStatus, h.getBody, `{"id":"ORDER1","status":"COMPLETED","purchase_units":[{"custom_id":"42","reference_id":"42","payments":{"captures":[{"id":"CAP1"}]}}]}`, http.StatusOK)
	case r.Method == http.MethodPost && r.URL.Path == "/v1/notifications/verify-webhook-signature":
		h.verifyCount++

		raw, _ := io.ReadAll(r.Body)
		h.verifyRequestBody = string(raw)

		h.write(w, h.verifyStatus, h.verifyBody, `{"verification_status":"SUCCESS"}`, http.StatusOK)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (h *paypalMockHandler) write(w http.ResponseWriter, status int, body, defaultBody string, defaultStatus int) {
	code := status
	if code == 0 {
		code = defaultStatus
	}

	payload := body
	if payload == "" {
		payload = defaultBody
	}

	w.WriteHeader(code)
	_, _ = io.WriteString(w, payload)
}

func newTestPaypalClient(t *testing.T, handler *paypalMockHandler) *PaypalClient {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client := NewPaypalClient("test-id", "test-secret", false)
	client.baseURL = server.URL

	return client
}

func TestPaypalClient_TokenCaching(t *testing.T) {
	t.Parallel()

	handler := &paypalMockHandler{}
	client := newTestPaypalClient(t, handler)

	first, err := client.token(context.Background())
	if err != nil {
		t.Fatalf("token: %v", err)
	}

	second, err := client.token(context.Background())
	if err != nil {
		t.Fatalf("token second call: %v", err)
	}

	if first != "test-token" || second != "test-token" {
		t.Errorf("token = (%q, %q), want both test-token", first, second)
	}
	if handler.tokenCount != 1 {
		t.Errorf("token endpoint hits = %d, want 1 (cached)", handler.tokenCount)
	}
}

func TestPaypalClient_TokenNon2xx(t *testing.T) {
	t.Parallel()

	handler := &paypalMockHandler{tokenStatus: http.StatusInternalServerError, tokenBody: `{"error":"server_error"}`}
	client := newTestPaypalClient(t, handler)

	_, err := client.token(context.Background())
	if err == nil {
		t.Fatal("token err = nil for a 500 response, want non-nil")
	}
}

func TestPaypalClient_CreateOrder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		createBody      string
		wantApprovalURL string
		wantErr         bool
	}{
		{
			name:            "approve link",
			wantApprovalURL: "https://www.sandbox.paypal.com/checkoutnow?token=ORDER1",
		},
		{
			name:            "payer-action fallback",
			createBody:      `{"id":"ORDER1","status":"CREATED","links":[{"rel":"self","href":"https://api/self"},{"rel":"payer-action","href":"https://payer/action"}]}`,
			wantApprovalURL: "https://payer/action",
		},
		{
			name:       "no usable link",
			createBody: `{"id":"ORDER1","status":"CREATED","links":[{"rel":"self","href":"https://api/self"}]}`,
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := &paypalMockHandler{createBody: tt.createBody}
			client := newTestPaypalClient(t, handler)

			result, err := client.CreateOrder(context.Background(), CreateOrderParams{ReferenceID: "42"})
			if tt.wantErr {
				if err == nil {
					t.Fatal("CreateOrder err = nil, want non-nil")
				}

				return
			}
			if err != nil {
				t.Fatalf("CreateOrder: %v", err)
			}

			if result.OrderID != "ORDER1" {
				t.Errorf("OrderID = %q, want ORDER1", result.OrderID)
			}
			if result.ApprovalURL != tt.wantApprovalURL {
				t.Errorf("ApprovalURL = %q, want %q", result.ApprovalURL, tt.wantApprovalURL)
			}
		})
	}
}

func TestPaypalClient_CaptureOrderHappy(t *testing.T) {
	t.Parallel()

	handler := &paypalMockHandler{}
	client := newTestPaypalClient(t, handler)

	result, err := client.CaptureOrder(context.Background(), "ORDER1")
	if err != nil {
		t.Fatalf("CaptureOrder: %v", err)
	}

	if result.CaptureID != "CAP1" {
		t.Errorf("CaptureID = %q, want CAP1", result.CaptureID)
	}
	if result.Status != paypalStatusCompleted {
		t.Errorf("Status = %q, want COMPLETED", result.Status)
	}
	if result.OrderID != "ORDER1" {
		t.Errorf("OrderID = %q, want ORDER1", result.OrderID)
	}
	if handler.captureRequestHeader != "ORDER1" {
		t.Errorf("PayPal-Request-Id header = %q, want ORDER1", handler.captureRequestHeader)
	}
}

func TestPaypalClient_CaptureOrderAlreadyCaptured(t *testing.T) {
	t.Parallel()

	handler := &paypalMockHandler{
		captureStatus: http.StatusUnprocessableEntity,
		captureBody:   `{"name":"UNPROCESSABLE_ENTITY","details":[{"issue":"ORDER_ALREADY_CAPTURED"}]}`,
	}
	client := newTestPaypalClient(t, handler)

	_, err := client.CaptureOrder(context.Background(), "ORDER1")
	if !errors.Is(err, ErrPaypalOrderAlreadyCaptured) {
		t.Fatalf("CaptureOrder err = %v, want ErrPaypalOrderAlreadyCaptured", err)
	}
}

func TestPaypalClient_CaptureOrderOther422(t *testing.T) {
	t.Parallel()

	handler := &paypalMockHandler{
		captureStatus: http.StatusUnprocessableEntity,
		captureBody:   `{"name":"UNPROCESSABLE_ENTITY","details":[{"issue":"INSTRUMENT_DECLINED"}]}`,
	}
	client := newTestPaypalClient(t, handler)

	_, err := client.CaptureOrder(context.Background(), "ORDER1")
	if err == nil {
		t.Fatal("CaptureOrder err = nil for INSTRUMENT_DECLINED, want non-nil")
	}
	if errors.Is(err, ErrPaypalOrderAlreadyCaptured) {
		t.Error("CaptureOrder err is ErrPaypalOrderAlreadyCaptured, want a different error")
	}
}

func TestPaypalClient_CaptureOrderMissingCaptures(t *testing.T) {
	t.Parallel()

	handler := &paypalMockHandler{captureBody: `{"id":"ORDER1","status":"COMPLETED","purchase_units":[{"payments":{"captures":[]}}]}`}
	client := newTestPaypalClient(t, handler)

	_, err := client.CaptureOrder(context.Background(), "ORDER1")
	if err == nil {
		t.Fatal("CaptureOrder err = nil for an empty captures array, want non-nil")
	}
}

func TestPaypalClient_GetOrder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		getBody         string
		wantReferenceID string
	}{
		{
			name:            "custom id preferred",
			wantReferenceID: "42",
		},
		{
			name:            "reference id fallback",
			getBody:         `{"id":"ORDER1","status":"COMPLETED","purchase_units":[{"custom_id":"","reference_id":"77","payments":{"captures":[{"id":"CAP1"}]}}]}`,
			wantReferenceID: "77",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := &paypalMockHandler{getBody: tt.getBody}
			client := newTestPaypalClient(t, handler)

			details, err := client.GetOrder(context.Background(), "ORDER1")
			if err != nil {
				t.Fatalf("GetOrder: %v", err)
			}

			if details.Status != paypalStatusCompleted {
				t.Errorf("Status = %q, want COMPLETED", details.Status)
			}
			if details.ReferenceID != tt.wantReferenceID {
				t.Errorf("ReferenceID = %q, want %q", details.ReferenceID, tt.wantReferenceID)
			}
			if details.CaptureID != "CAP1" {
				t.Errorf("CaptureID = %q, want CAP1", details.CaptureID)
			}
		})
	}
}

func TestPaypalClient_VerifyWebhook(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		verifyBody   string
		verifyStatus int
		want         bool
		wantErr      bool
	}{
		{name: "success", verifyBody: `{"verification_status":"SUCCESS"}`, want: true},
		{name: "failure", verifyBody: `{"verification_status":"FAILURE"}`, want: false},
		{name: "non 2xx", verifyStatus: http.StatusInternalServerError, verifyBody: `{"error":"boom"}`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := &paypalMockHandler{verifyStatus: tt.verifyStatus, verifyBody: tt.verifyBody}
			client := newTestPaypalClient(t, handler)

			got, err := client.VerifyWebhook(context.Background(), WebhookSignatureParams{
				WebhookID: "WH-123",
				Event:     json.RawMessage(`{"id":"evt_1","resource":{"id":"r1"}}`),
			})
			if tt.wantErr {
				if err == nil {
					t.Fatal("VerifyWebhook err = nil, want non-nil")
				}

				return
			}
			if err != nil {
				t.Fatalf("VerifyWebhook: %v", err)
			}

			if got != tt.want {
				t.Errorf("VerifyWebhook = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPaypalClient_VerifyWebhookRequestBody(t *testing.T) {
	t.Parallel()

	handler := &paypalMockHandler{}
	client := newTestPaypalClient(t, handler)

	_, err := client.VerifyWebhook(context.Background(), WebhookSignatureParams{
		WebhookID: "WH-123",
		Event:     json.RawMessage(`{"id":"evt_1","resource":{"id":"r1"}}`),
	})
	if err != nil {
		t.Fatalf("VerifyWebhook: %v", err)
	}

	if !strings.Contains(handler.verifyRequestBody, `"webhook_id":"WH-123"`) {
		t.Errorf("verify request body = %q, want it to contain the webhook_id", handler.verifyRequestBody)
	}
	if !strings.Contains(handler.verifyRequestBody, `"webhook_event":{"id":"evt_1","resource":{"id":"r1"}}`) {
		t.Errorf("verify request body = %q, want the raw webhook_event embedded verbatim", handler.verifyRequestBody)
	}
}

func TestNewPaypalError_HasIssue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		code string
		want bool
	}{
		{name: "issue at details index 0", body: `{"name":"UNPROCESSABLE_ENTITY","details":[{"issue":"ORDER_ALREADY_CAPTURED"}]}`, code: "ORDER_ALREADY_CAPTURED", want: true},
		{name: "issue at details index 1", body: `{"name":"UNPROCESSABLE_ENTITY","details":[{"issue":"SOMETHING_ELSE"},{"issue":"ORDER_ALREADY_CAPTURED"}]}`, code: "ORDER_ALREADY_CAPTURED", want: true},
		{name: "code in top level name", body: `{"name":"ORDER_ALREADY_CAPTURED"}`, code: "ORDER_ALREADY_CAPTURED", want: true},
		{name: "code absent", body: `{"name":"UNPROCESSABLE_ENTITY","details":[{"issue":"INSTRUMENT_DECLINED"}]}`, code: "ORDER_ALREADY_CAPTURED", want: false},
		{name: "invalid json", body: `not json`, code: "ORDER_ALREADY_CAPTURED", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			apiErr := newPaypalError(http.StatusUnprocessableEntity, []byte(tt.body))
			if got := apiErr.hasIssue(tt.code); got != tt.want {
				t.Errorf("hasIssue(%q) = %v, want %v", tt.code, got, tt.want)
			}

			message := apiErr.Error()
			if !strings.Contains(message, "422") {
				t.Errorf("Error() = %q, want it to include the status 422", message)
			}
			if !strings.Contains(message, tt.body) {
				t.Errorf("Error() = %q, want it to include the raw body %q", message, tt.body)
			}
		})
	}
}

func TestPaypalApprovalURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		want  string
		links []struct {
			Href string `json:"href"`
			Rel  string `json:"rel"`
		}
	}{
		{
			name: "approve present",
			links: []struct {
				Href string `json:"href"`
				Rel  string `json:"rel"`
			}{{Rel: "self", Href: "https://api/self"}, {Rel: "approve", Href: "https://approve"}},
			want: "https://approve",
		},
		{
			name: "only payer-action",
			links: []struct {
				Href string `json:"href"`
				Rel  string `json:"rel"`
			}{{Rel: "self", Href: "https://api/self"}, {Rel: "payer-action", Href: "https://payer"}},
			want: "https://payer",
		},
		{
			name: "neither",
			links: []struct {
				Href string `json:"href"`
				Rel  string `json:"rel"`
			}{{Rel: "self", Href: "https://api/self"}},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := paypalApprovalURL(tt.links); got != tt.want {
				t.Errorf("paypalApprovalURL = %q, want %q", got, tt.want)
			}
		})
	}
}
