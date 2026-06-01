package infra

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type nowpaymentsMockHandler struct {
	invoiceBody        string
	invoiceRequestBody string
	invoiceAPIKey      string
	invoiceStatus      int
	invoiceCount       int
	mu                 sync.Mutex
}

func (h *nowpaymentsMockHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()

	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/v1/invoice":
		h.invoiceCount++
		h.invoiceAPIKey = r.Header.Get("x-api-key")

		raw, _ := io.ReadAll(r.Body)
		h.invoiceRequestBody = string(raw)

		h.write(w, h.invoiceStatus, h.invoiceBody, `{"id":"INV1","invoice_url":"https://sandbox.nowpayments.io/payment/?iid=INV1"}`, http.StatusOK)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (h *nowpaymentsMockHandler) write(w http.ResponseWriter, status int, body, defaultBody string, defaultStatus int) {
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

func newTestNowPaymentsClient(t *testing.T, handler *nowpaymentsMockHandler) *NowPaymentsClient {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client := NewNowPaymentsClient("test-api-key", "test-ipn-secret", false)
	client.baseURL = server.URL

	return client
}

func signNowpaymentsIPN(t *testing.T, secret, canonical string) string {
	t.Helper()

	mac := hmac.New(sha512.New, []byte(secret))
	mac.Write([]byte(canonical))

	return hex.EncodeToString(mac.Sum(nil))
}

func TestNowPaymentsClient_CreateInvoice(t *testing.T) {
	t.Parallel()

	handler := &nowpaymentsMockHandler{}
	client := newTestNowPaymentsClient(t, handler)

	result, err := client.CreateInvoice(context.Background(), CreateInvoiceParams{
		PriceAmount:      5,
		PriceCurrency:    "usd",
		OrderID:          "42",
		OrderDescription: "Starter",
		SuccessURL:       "https://app.test/ok",
		CancelURL:        "https://app.test/cancel",
		IPNCallbackURL:   "https://app.test/webhooks/nowpayments",
	})
	if err != nil {
		t.Fatalf("CreateInvoice: %v", err)
	}

	if result.InvoiceID != "INV1" {
		t.Errorf("InvoiceID = %q, want INV1", result.InvoiceID)
	}
	if !strings.Contains(result.InvoiceURL, "iid=INV1") {
		t.Errorf("InvoiceURL = %q, want it to contain iid=INV1", result.InvoiceURL)
	}
	if handler.invoiceAPIKey != "test-api-key" {
		t.Errorf("x-api-key header = %q, want test-api-key", handler.invoiceAPIKey)
	}
	if !strings.Contains(handler.invoiceRequestBody, `"order_id":"42"`) {
		t.Errorf("invoice request body = %q, want order_id 42", handler.invoiceRequestBody)
	}
	if !strings.Contains(handler.invoiceRequestBody, `"price_currency":"usd"`) {
		t.Errorf("invoice request body = %q, want price_currency usd", handler.invoiceRequestBody)
	}
	if !strings.Contains(handler.invoiceRequestBody, `"price_amount":5`) {
		t.Errorf("invoice request body = %q, want price_amount 5", handler.invoiceRequestBody)
	}
	if !strings.Contains(handler.invoiceRequestBody, `"ipn_callback_url":"https://app.test/webhooks/nowpayments"`) {
		t.Errorf("invoice request body = %q, want ipn_callback_url", handler.invoiceRequestBody)
	}
}

func TestNowPaymentsClient_CreateInvoiceNon2xx(t *testing.T) {
	t.Parallel()

	handler := &nowpaymentsMockHandler{invoiceStatus: http.StatusBadRequest, invoiceBody: `{"error":"bad"}`}
	client := newTestNowPaymentsClient(t, handler)

	_, err := client.CreateInvoice(context.Background(), CreateInvoiceParams{})
	if err == nil {
		t.Fatal("CreateInvoice err = nil for a 400 response, want non-nil")
	}
}

func TestNowPaymentsClient_CreateInvoiceMalformedBody(t *testing.T) {
	t.Parallel()

	handler := &nowpaymentsMockHandler{invoiceBody: "not json"}
	client := newTestNowPaymentsClient(t, handler)

	_, err := client.CreateInvoice(context.Background(), CreateInvoiceParams{})
	if err == nil {
		t.Fatal("CreateInvoice err = nil for a malformed body, want non-nil")
	}
}

func TestNowPaymentsClient_VerifyIPN(t *testing.T) {
	t.Parallel()

	client := NewNowPaymentsClient("ignored", "test-ipn-secret", false)

	tests := []struct {
		name              string
		body              string
		canonical         string
		signatureOverride string
		useGivenSignature bool
		want              bool
		wantErr           bool
	}{
		{name: "keys already sorted", body: `{"order_id":"42","payment_status":"finished"}`, canonical: `{"order_id":"42","payment_status":"finished"}`, want: true},
		{name: "keys unsorted resorted", body: `{"payment_status":"finished","order_id":"42"}`, canonical: `{"order_id":"42","payment_status":"finished"}`, want: true},
		{name: "decimal value preserved", body: `{"order_id":"42","pay_amount":0.40}`, canonical: `{"order_id":"42","pay_amount":0.40}`, want: true},
		{name: "numeric payment_id", body: `{"order_id":"42","payment_id":5667108377}`, canonical: `{"order_id":"42","payment_id":5667108377}`, want: true},
		{name: "tampered body", body: `{"order_id":"99"}`, canonical: `{"order_id":"42"}`, want: false},
		{name: "wrong signature", body: `{"order_id":"42"}`, signatureOverride: "deadbeef", useGivenSignature: true, want: false},
		{name: "empty signature", body: `{"order_id":"42"}`, signatureOverride: "", useGivenSignature: true, want: false},
		{name: "malformed json", body: `not json`, signatureOverride: "anything", useGivenSignature: true, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			signature := tt.signatureOverride
			if !tt.useGivenSignature {
				signature = signNowpaymentsIPN(t, "test-ipn-secret", tt.canonical)
			}

			got, err := client.VerifyIPN(signature, []byte(tt.body))
			if tt.wantErr {
				if err == nil {
					t.Fatal("VerifyIPN err = nil, want non-nil")
				}

				return
			}
			if err != nil {
				t.Fatalf("VerifyIPN: %v", err)
			}

			if got != tt.want {
				t.Errorf("VerifyIPN = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNowPaymentsClient_VerifyIPNEmptySecret(t *testing.T) {
	t.Parallel()

	client := NewNowPaymentsClient("k", "", false)

	got, err := client.VerifyIPN("whatever", []byte(`{"order_id":"42"}`))
	if err != nil {
		t.Fatalf("VerifyIPN: %v", err)
	}
	if got {
		t.Errorf("VerifyIPN = %v, want false for an empty secret", got)
	}
}
