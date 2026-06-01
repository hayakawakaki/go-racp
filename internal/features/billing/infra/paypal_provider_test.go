package infra

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/hayakawakaki/go-racp/internal/features/billing/domain"
)

func TestPaypalOrderIDPattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "uppercase alnum", input: "5O190127TN364715T", want: true},
		{name: "with hyphen", input: "EC-1AB23", want: true},
		{name: "empty", input: "", want: false},
		{name: "with slash", input: "a/b", want: false},
		{name: "with dot", input: "a..b", want: false},
		{name: "with question mark", input: "a?b", want: false},
		{name: "with space", input: "a b", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := paypalOrderIDPattern.MatchString(tt.input); got != tt.want {
				t.Errorf("MatchString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestPaypalProvider_Name(t *testing.T) {
	t.Parallel()

	if got := NewPaypalProvider(nil).Name(); got != "paypal" {
		t.Errorf("Name() = %q, want paypal", got)
	}
}

func TestPaypalProvider_CreateCheckout(t *testing.T) {
	t.Parallel()

	handler := &paypalMockHandler{}
	client := newTestPaypalClient(t, handler)
	provider := NewPaypalProvider(client)

	result, err := provider.CreateCheckout(context.Background(), domain.CheckoutRequest{
		PurchaseID:  42,
		Amount:      5,
		Currency:    "USD",
		Description: "Starter",
		SuccessURL:  "https://app.test/ok",
		CancelURL:   "https://app.test/cancel",
	})
	if err != nil {
		t.Fatalf("CreateCheckout: %v", err)
	}

	if result.RedirectURL != "https://www.sandbox.paypal.com/checkoutnow?token=ORDER1" {
		t.Errorf("RedirectURL = %q, want the approval link", result.RedirectURL)
	}
	if result.Reference != "ORDER1" {
		t.Errorf("Reference = %q, want ORDER1", result.Reference)
	}
	if !strings.Contains(handler.createRequestBody, `"value":"5.00"`) {
		t.Errorf("create request body = %q, want amount value 5.00", handler.createRequestBody)
	}
	if !strings.Contains(handler.createRequestBody, `"reference_id":"42"`) {
		t.Errorf("create request body = %q, want reference_id 42", handler.createRequestBody)
	}
	if !strings.Contains(handler.createRequestBody, `"custom_id":"42"`) {
		t.Errorf("create request body = %q, want custom_id 42", handler.createRequestBody)
	}
}

func TestPaypalProvider_CreateCheckoutUnsupportedCurrency(t *testing.T) {
	t.Parallel()

	handler := &paypalMockHandler{}
	client := newTestPaypalClient(t, handler)
	provider := NewPaypalProvider(client)

	_, err := provider.CreateCheckout(context.Background(), domain.CheckoutRequest{
		PurchaseID: 42,
		Amount:     5,
		Currency:   "GBP",
	})
	if err == nil {
		t.Fatal("CreateCheckout with GBP error = nil, want unsupported currency error")
	}
	if handler.createCount != 0 {
		t.Errorf("create endpoint hits = %d, want 0 for an unsupported currency", handler.createCount)
	}
}

func TestPaypalProvider_RetrieveCheckoutFreshCapture(t *testing.T) {
	t.Parallel()

	handler := &paypalMockHandler{}
	client := newTestPaypalClient(t, handler)
	provider := NewPaypalProvider(client)

	confirmation, err := provider.RetrieveCheckout(context.Background(), url.Values{"token": {"ORDER1"}})
	if err != nil {
		t.Fatalf("RetrieveCheckout: %v", err)
	}

	if confirmation.PurchaseID != 42 {
		t.Errorf("PurchaseID = %d, want 42", confirmation.PurchaseID)
	}
	if !confirmation.Paid {
		t.Error("Paid = false, want true")
	}
}

func TestPaypalProvider_RetrieveCheckoutAlreadyCaptured(t *testing.T) {
	t.Parallel()

	handler := &paypalMockHandler{
		captureStatus: http.StatusUnprocessableEntity,
		captureBody:   `{"name":"UNPROCESSABLE_ENTITY","details":[{"issue":"ORDER_ALREADY_CAPTURED"}]}`,
	}
	client := newTestPaypalClient(t, handler)
	provider := NewPaypalProvider(client)

	confirmation, err := provider.RetrieveCheckout(context.Background(), url.Values{"token": {"ORDER1"}})
	if err != nil {
		t.Fatalf("RetrieveCheckout: %v", err)
	}

	if confirmation.PurchaseID != 42 {
		t.Errorf("PurchaseID = %d, want 42", confirmation.PurchaseID)
	}
	if !confirmation.Paid {
		t.Error("Paid = false, want true")
	}
	if handler.getCount != 1 {
		t.Errorf("get order endpoint hits = %d, want 1 (fallback path)", handler.getCount)
	}
}

func TestPaypalProvider_RetrieveCheckoutInvalidToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		values url.Values
		name   string
	}{
		{name: "no token", values: url.Values{}},
		{name: "path traversal token", values: url.Values{"token": {"../evil"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := &paypalMockHandler{}
			client := newTestPaypalClient(t, handler)
			provider := NewPaypalProvider(client)

			confirmation, err := provider.RetrieveCheckout(context.Background(), tt.values)
			if err != nil {
				t.Fatalf("RetrieveCheckout: %v", err)
			}

			if confirmation != (domain.CheckoutConfirmation{}) {
				t.Errorf("confirmation = %+v, want zero value", confirmation)
			}
			if handler.tokenCount != 0 || handler.captureCount != 0 || handler.getCount != 0 {
				t.Errorf("http hits token=%d capture=%d get=%d, want all 0 for an invalid token", handler.tokenCount, handler.captureCount, handler.getCount)
			}
		})
	}
}

func TestPaypalProvider_RetrieveCheckoutCaptureDeclined(t *testing.T) {
	t.Parallel()

	handler := &paypalMockHandler{captureStatus: http.StatusInternalServerError, captureBody: `{"error":"boom"}`}
	client := newTestPaypalClient(t, handler)
	provider := NewPaypalProvider(client)

	_, err := provider.RetrieveCheckout(context.Background(), url.Values{"token": {"ORDER1"}})
	if err == nil {
		t.Fatal("RetrieveCheckout err = nil for a 500 capture, want non-nil")
	}
}

func TestPaypalProvider_Capture(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		captureBody   string
		want          domain.CaptureOutcome
		captureStatus int
		wantErr       bool
	}{
		{
			name: "success",
			want: domain.CaptureOutcome{PaymentID: "CAP1", Completed: true},
		},
		{
			name:          "already captured",
			captureStatus: http.StatusUnprocessableEntity,
			captureBody:   `{"name":"UNPROCESSABLE_ENTITY","details":[{"issue":"ORDER_ALREADY_CAPTURED"}]}`,
			want:          domain.CaptureOutcome{},
		},
		{
			name:          "other error",
			captureStatus: http.StatusInternalServerError,
			captureBody:   `{"error":"boom"}`,
			wantErr:       true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := &paypalMockHandler{captureStatus: tt.captureStatus, captureBody: tt.captureBody}
			client := newTestPaypalClient(t, handler)
			provider := NewPaypalProvider(client)

			outcome, err := provider.Capture(context.Background(), "ORDER1")
			if tt.wantErr {
				if err == nil {
					t.Fatal("Capture err = nil, want non-nil")
				}

				return
			}
			if err != nil {
				t.Fatalf("Capture: %v", err)
			}

			if outcome != tt.want {
				t.Errorf("Capture = %+v, want %+v", outcome, tt.want)
			}
		})
	}
}
