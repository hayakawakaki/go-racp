package infra

import (
	"bytes"
	"context"
	"testing"

	"github.com/hayakawakaki/go-racp/internal/features/billing/domain"
	"github.com/stripe/stripe-go/v85"
)

type captureBackend struct {
	params stripe.ParamsContainer
}

var _ stripe.Backend = (*captureBackend)(nil)

func (b *captureBackend) Call(_, _, _ string, params stripe.ParamsContainer, v stripe.LastResponseSetter) error {
	b.params = params
	if sess, ok := v.(*stripe.CheckoutSession); ok {
		sess.ID = "cs_test_123"
		sess.URL = "https://checkout.stripe.com/c/pay/cs_test_123"
	}

	return nil
}

func (b *captureBackend) CallStreaming(_, _, _ string, _ stripe.ParamsContainer, _ stripe.StreamingLastResponseSetter) error {
	return nil
}

func (b *captureBackend) CallRaw(_, _, _ string, _ []byte, _ *stripe.Params, _ stripe.LastResponseSetter) error {
	return nil
}

func (b *captureBackend) CallMultipart(_, _, _, _ string, _ *bytes.Buffer, _ *stripe.Params, _ stripe.LastResponseSetter) error {
	return nil
}

func (b *captureBackend) SetMaxNetworkRetries(_ int64) {}

func TestStripeProvider_Name(t *testing.T) {
	if got := NewStripeProvider("sk_test_dummy").Name(); got != "stripe" {
		t.Errorf("Name() = %q, want stripe", got)
	}
}

func TestStripeProvider_CreateCheckout_MapsParams(t *testing.T) {
	backend := &captureBackend{}
	stripe.SetBackend(stripe.APIBackend, backend)

	provider := NewStripeProvider("sk_test_dummy")
	result, err := provider.CreateCheckout(context.Background(), domain.CheckoutRequest{
		PackageKey:  "starter",
		Description: "Starter Pack",
		Currency:    "USD",
		SuccessURL:  "https://app.test/ok",
		CancelURL:   "https://app.test/cancel",
		PurchaseID:  9,
		Amount:      5,
	})
	if err != nil {
		t.Fatalf("CreateCheckout: %v", err)
	}
	if result.RedirectURL != "https://checkout.stripe.com/c/pay/cs_test_123" || result.Reference != "cs_test_123" {
		t.Fatalf("result = %+v, want canned redirect url and reference", result)
	}

	params, ok := backend.params.(*stripe.CheckoutSessionParams)
	if !ok {
		t.Fatalf("captured params type = %T, want *stripe.CheckoutSessionParams", backend.params)
	}
	if len(params.LineItems) != 1 {
		t.Fatalf("line items = %d, want 1", len(params.LineItems))
	}

	price := params.LineItems[0].PriceData
	if price == nil || price.UnitAmount == nil || *price.UnitAmount != 500 {
		t.Errorf("unit amount = %v, want 500 (5 USD in cents)", price.UnitAmount)
	}
	if price.Currency == nil || *price.Currency != "USD" {
		t.Errorf("currency = %v, want USD", price.Currency)
	}
	if params.Metadata["purchase_id"] != "9" {
		t.Errorf("metadata purchase_id = %q, want 9", params.Metadata["purchase_id"])
	}
}

func TestStripeProvider_CreateCheckout_RejectsUnsupportedCurrency(t *testing.T) {
	provider := NewStripeProvider("sk_test_dummy")
	_, err := provider.CreateCheckout(context.Background(), domain.CheckoutRequest{
		Currency:   "GBP",
		Amount:     5,
		PurchaseID: 9,
	})
	if err == nil {
		t.Fatal("CreateCheckout with GBP error = nil, want unsupported currency error")
	}
}
