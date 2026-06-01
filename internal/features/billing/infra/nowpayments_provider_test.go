package infra

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/hayakawakaki/go-racp/internal/features/billing/domain"
)

func TestNowPaymentsProvider_Name(t *testing.T) {
	t.Parallel()

	if got := NewNowPaymentsProvider(nil, "").Name(); got != "crypto" {
		t.Errorf("Name() = %q, want crypto", got)
	}
}

func TestNowPaymentsProvider_IsAsyncConfirmer(t *testing.T) {
	t.Parallel()

	if _, ok := any(NewNowPaymentsProvider(nil, "")).(domain.AsyncConfirmer); !ok {
		t.Error("NowPaymentsProvider does not implement domain.AsyncConfirmer, want it to")
	}
}

func TestNowPaymentsProvider_CreateCheckout(t *testing.T) {
	t.Parallel()

	handler := &nowpaymentsMockHandler{}
	client := newTestNowPaymentsClient(t, handler)
	provider := NewNowPaymentsProvider(client, "https://app.test/webhooks/nowpayments")

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

	if result.Reference != "INV1" {
		t.Errorf("Reference = %q, want INV1", result.Reference)
	}
	if !strings.Contains(result.RedirectURL, "iid=INV1") {
		t.Errorf("RedirectURL = %q, want it to contain iid=INV1", result.RedirectURL)
	}
	if !strings.Contains(handler.invoiceRequestBody, `"price_currency":"usd"`) {
		t.Errorf("invoice request body = %q, want lowercased price_currency usd", handler.invoiceRequestBody)
	}
	if !strings.Contains(handler.invoiceRequestBody, `"order_id":"42"`) {
		t.Errorf("invoice request body = %q, want order_id 42", handler.invoiceRequestBody)
	}
	if !strings.Contains(handler.invoiceRequestBody, `"ipn_callback_url":"https://app.test/webhooks/nowpayments"`) {
		t.Errorf("invoice request body = %q, want ipn_callback_url", handler.invoiceRequestBody)
	}
}

func TestNowPaymentsProvider_CreateCheckoutError(t *testing.T) {
	t.Parallel()

	handler := &nowpaymentsMockHandler{invoiceStatus: http.StatusBadRequest}
	client := newTestNowPaymentsClient(t, handler)
	provider := NewNowPaymentsProvider(client, "https://app.test/webhooks/nowpayments")

	_, err := provider.CreateCheckout(context.Background(), domain.CheckoutRequest{PurchaseID: 42, Amount: 5, Currency: "USD"})
	if err == nil {
		t.Fatal("CreateCheckout err = nil for a 400 invoice, want non-nil")
	}
}

func TestNowPaymentsProvider_RetrieveCheckout(t *testing.T) {
	t.Parallel()

	provider := NewNowPaymentsProvider(nil, "")

	confirmation, err := provider.RetrieveCheckout(context.Background(), url.Values{})
	if err != nil {
		t.Fatalf("RetrieveCheckout: %v", err)
	}
	if confirmation != (domain.CheckoutConfirmation{}) {
		t.Errorf("confirmation = %+v, want zero value", confirmation)
	}
}
