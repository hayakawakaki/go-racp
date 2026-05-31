package transport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/billing/domain"
	"github.com/stripe/stripe-go/v85/webhook"
)

const testWebhookSecret = "whsec_test_secret"

type completeCall struct {
	paymentID  string
	purchaseID int64
}

type paymentCall struct {
	provider  string
	paymentID string
}

type webhookStub struct {
	completeErr error
	refundErr   error
	disputeErr  error
	failErr     error
	completed   []completeCall
	refunded    []paymentCall
	disputed    []paymentCall
	failed      []int64
}

var _ billingService = (*webhookStub)(nil)

func (s *webhookStub) Packages() []domain.Package { return nil }

func (s *webhookStub) Available() bool { return true }

func (s *webhookStub) StartCheckout(context.Context, int, string, string, string) (string, error) {
	return "", nil
}

func (s *webhookStub) HistoryByAccount(context.Context, int, int) ([]domain.Purchase, error) {
	return nil, nil
}

func (s *webhookStub) ConfirmCheckout(context.Context, string, int) (domain.Package, bool, error) {
	return domain.Package{}, false, nil
}

func (s *webhookStub) CompletePurchase(_ context.Context, purchaseID int64, paymentID string) error {
	s.completed = append(s.completed, completeCall{paymentID, purchaseID})

	return s.completeErr
}

func (s *webhookStub) RefundPurchase(_ context.Context, provider, paymentID string) error {
	s.refunded = append(s.refunded, paymentCall{provider, paymentID})

	return s.refundErr
}

func (s *webhookStub) DisputePurchase(_ context.Context, provider, paymentID string) error {
	s.disputed = append(s.disputed, paymentCall{provider, paymentID})

	return s.disputeErr
}

func (s *webhookStub) FailPurchase(_ context.Context, purchaseID int64) error {
	s.failed = append(s.failed, purchaseID)

	return s.failErr
}

func newWebhookHandler(svc billingService, secret string) *Handler {
	return NewHandler(svc, HandlerConfig{
		Logger:              discardLogger(),
		StripeWebhookSecret: secret,
	})
}

func signedRequest(secret, body string) *http.Request {
	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload: []byte(body),
		Secret:  secret,
	})
	req := httptest.NewRequest(http.MethodPost, "/webhooks/stripe", bytes.NewReader(signed.Payload))
	req.Header.Set("Stripe-Signature", signed.Header)

	return req
}

func completedEvent(created int64, paymentStatus, purchaseID, paymentIntent string) string {
	return fmt.Sprintf(`{"id":"evt_1","object":"event","type":"checkout.session.completed","created":%d,"data":{"object":{"id":"cs_1","payment_status":%q,"metadata":{"purchase_id":%q},"payment_intent":%q}}}`,
		created, paymentStatus, purchaseID, paymentIntent)
}

func chargeEvent(eventType string, created int64, paymentIntent string) string {
	return fmt.Sprintf(`{"id":"evt_1","object":"event","type":%q,"created":%d,"data":{"object":{"id":"ch_1","payment_intent":%q}}}`,
		eventType, created, paymentIntent)
}

func disputeEvent(created int64, paymentIntent string) string {
	return fmt.Sprintf(`{"id":"evt_1","object":"event","type":"charge.dispute.created","created":%d,"data":{"object":{"id":"dp_1","payment_intent":%q}}}`,
		created, paymentIntent)
}

func TestStripeWebhook_CompletedPaidCredits(t *testing.T) {
	t.Parallel()

	svc := &webhookStub{}
	h := newWebhookHandler(svc, testWebhookSecret)

	rr := httptest.NewRecorder()
	h.stripeWebhook(rr, signedRequest(testWebhookSecret, completedEvent(time.Now().Unix(), "paid", "9", "pi_test_1")))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if len(svc.completed) != 1 || svc.completed[0].purchaseID != 9 || svc.completed[0].paymentID != "pi_test_1" {
		t.Errorf("completed = %+v, want one call for purchase 9 with pi_test_1", svc.completed)
	}
}

func TestStripeWebhook_InvalidSignatureRejected(t *testing.T) {
	t.Parallel()

	svc := &webhookStub{}
	h := newWebhookHandler(svc, testWebhookSecret)

	rr := httptest.NewRecorder()
	h.stripeWebhook(rr, signedRequest("whsec_wrong_secret", completedEvent(time.Now().Unix(), "paid", "9", "pi_test_1")))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	if len(svc.completed) != 0 {
		t.Errorf("completed = %+v, want none on bad signature", svc.completed)
	}
}

func TestStripeWebhook_UnconfiguredReturns503(t *testing.T) {
	t.Parallel()

	h := newWebhookHandler(&webhookStub{}, "")

	rr := httptest.NewRecorder()
	h.stripeWebhook(rr, httptest.NewRequest(http.MethodPost, "/webhooks/stripe", strings.NewReader("{}")))

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rr.Code)
	}
}

func TestStripeWebhook_UnpaidSessionSkipped(t *testing.T) {
	t.Parallel()

	svc := &webhookStub{}
	h := newWebhookHandler(svc, testWebhookSecret)

	rr := httptest.NewRecorder()
	h.stripeWebhook(rr, signedRequest(testWebhookSecret, completedEvent(time.Now().Unix(), "unpaid", "9", "pi_test_1")))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if len(svc.completed) != 0 {
		t.Errorf("completed = %+v, want none for an unpaid session", svc.completed)
	}
}

func TestStripeWebhook_MissingDataObjectReturns200(t *testing.T) {
	t.Parallel()

	svc := &webhookStub{}
	h := newWebhookHandler(svc, testWebhookSecret)

	body := fmt.Sprintf(`{"id":"evt_1","object":"event","type":"checkout.session.completed","created":%d}`, time.Now().Unix())
	rr := httptest.NewRecorder()
	h.stripeWebhook(rr, signedRequest(testWebhookSecret, body))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if len(svc.completed) != 0 {
		t.Errorf("completed = %+v, want none when the event has no data object", svc.completed)
	}
}

func TestStripeWebhook_RefundMarksRefundedNeverDisputes(t *testing.T) {
	t.Parallel()

	svc := &webhookStub{}
	h := newWebhookHandler(svc, testWebhookSecret)

	rr := httptest.NewRecorder()
	h.stripeWebhook(rr, signedRequest(testWebhookSecret, chargeEvent("charge.refunded", time.Now().Unix(), "pi_test_1")))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if len(svc.refunded) != 1 || svc.refunded[0].provider != "stripe" || svc.refunded[0].paymentID != "pi_test_1" {
		t.Errorf("refunded = %+v, want one call for stripe pi_test_1", svc.refunded)
	}
	if len(svc.disputed) != 0 {
		t.Errorf("disputed = %+v, want none on a refund", svc.disputed)
	}
}

func TestStripeWebhook_DisputeBans(t *testing.T) {
	t.Parallel()

	svc := &webhookStub{}
	h := newWebhookHandler(svc, testWebhookSecret)

	rr := httptest.NewRecorder()
	h.stripeWebhook(rr, signedRequest(testWebhookSecret, disputeEvent(time.Now().Unix(), "pi_test_1")))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if len(svc.disputed) != 1 || svc.disputed[0].paymentID != "pi_test_1" {
		t.Errorf("disputed = %+v, want one call for pi_test_1", svc.disputed)
	}
}

func TestStripeWebhook_UnhandledEventNoop(t *testing.T) {
	t.Parallel()

	svc := &webhookStub{}
	h := newWebhookHandler(svc, testWebhookSecret)

	body := fmt.Sprintf(`{"id":"evt_1","object":"event","type":"payment_intent.created","created":%d,"data":{"object":{"id":"pi_1"}}}`, time.Now().Unix())
	rr := httptest.NewRecorder()
	h.stripeWebhook(rr, signedRequest(testWebhookSecret, body))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if len(svc.completed)+len(svc.refunded)+len(svc.disputed)+len(svc.failed) != 0 {
		t.Errorf("no fulfiller call expected for an unhandled event type")
	}
}

func TestStripeWebhook_RefundNotFoundWithinWindowRetries(t *testing.T) {
	t.Parallel()

	svc := &webhookStub{refundErr: domain.ErrPurchaseNotFound}
	h := newWebhookHandler(svc, testWebhookSecret)

	rr := httptest.NewRecorder()
	h.stripeWebhook(rr, signedRequest(testWebhookSecret, chargeEvent("charge.refunded", time.Now().Unix(), "pi_test_1")))

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 so Stripe retries until completion lands", rr.Code)
	}
}

func TestStripeWebhook_RefundNotFoundPastWindowGivesUp(t *testing.T) {
	t.Parallel()

	svc := &webhookStub{refundErr: domain.ErrPurchaseNotFound}
	h := newWebhookHandler(svc, testWebhookSecret)

	old := time.Now().Add(-30 * time.Minute).Unix()
	rr := httptest.NewRecorder()
	h.stripeWebhook(rr, signedRequest(testWebhookSecret, chargeEvent("charge.refunded", old, "pi_test_1")))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 after the retry window elapsed", rr.Code)
	}
}

func TestStripeWebhook_CompletionTransientErrorRetries(t *testing.T) {
	t.Parallel()

	svc := &webhookStub{completeErr: errors.New("db unavailable")}
	h := newWebhookHandler(svc, testWebhookSecret)

	rr := httptest.NewRecorder()
	h.stripeWebhook(rr, signedRequest(testWebhookSecret, completedEvent(time.Now().Unix(), "paid", "9", "pi_test_1")))

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 on a transient fulfillment error", rr.Code)
	}
}

func TestPurchaseIDFromMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		metadata map[string]string
		name     string
		want     int64
		wantOK   bool
	}{
		{name: "valid id", metadata: map[string]string{"purchase_id": "42"}, want: 42, wantOK: true},
		{name: "missing key", metadata: map[string]string{}, wantOK: false},
		{name: "non numeric", metadata: map[string]string{"purchase_id": "abc"}, wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := purchaseIDFromMetadata(tt.metadata)
			if ok != tt.wantOK || got != tt.want {
				t.Errorf("purchaseIDFromMetadata(%v) = (%d, %v), want (%d, %v)", tt.metadata, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestIsTerminalFulfillmentError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err  error
		name string
		want bool
	}{
		{name: "purchase not found is terminal", err: domain.ErrPurchaseNotFound, want: true},
		{name: "unknown package is terminal", err: domain.ErrUnknownPackage, want: true},
		{name: "wrapped not found is terminal", err: fmt.Errorf("wrap: %w", domain.ErrPurchaseNotFound), want: true},
		{name: "generic error is transient", err: errors.New("db unavailable"), want: false},
		{name: "nil is not terminal", err: nil, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isTerminalFulfillmentError(tt.err); got != tt.want {
				t.Errorf("isTerminalFulfillmentError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
