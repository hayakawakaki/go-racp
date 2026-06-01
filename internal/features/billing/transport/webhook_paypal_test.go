package transport

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/billing/domain"
	"github.com/hayakawakaki/go-racp/internal/features/billing/infra"
)

const testPaypalWebhookID = "WH-PAYPAL-TEST"

type fakeVerifier struct {
	err      error
	calls    int
	verified bool
}

func (v *fakeVerifier) VerifyWebhook(_ context.Context, _ infra.WebhookSignatureParams) (bool, error) {
	v.calls++

	return v.verified, v.err
}

func newPaypalHandler(svc billingService, v paypalVerifier, webhookID string) *Handler {
	return NewHandler(svc, HandlerConfig{Logger: discardLogger(), Paypal: v, PaypalWebhookID: webhookID})
}

func paypalRequest(body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/webhooks/paypal", strings.NewReader(body))
	req.Header.Set("Paypal-Transmission-Id", "tx-1")
	req.Header.Set("Paypal-Transmission-Sig", "sig-1")
	req.Header.Set("Paypal-Transmission-Time", "2026-06-01T00:00:00Z")
	req.Header.Set("Paypal-Auth-Algo", "SHA256withRSA")
	req.Header.Set("Paypal-Cert-Url", "https://api.paypal.com/cert")

	return req
}

func paypalEnvelope(eventType, createTime, resource string) string {
	return fmt.Sprintf(`{"id":"WH-1","event_type":%q,"create_time":%q,"resource":%s}`,
		eventType, createTime, resource)
}

func approvedResource(customID string) string {
	unit := "{}"
	if customID != "" {
		unit = fmt.Sprintf(`{"custom_id":%q}`, customID)
	}

	return fmt.Sprintf(`{"id":"ORDER1","purchase_units":[%s]}`, unit)
}

func captureResource(status string) string {
	return fmt.Sprintf(`{"id":"CAP1","custom_id":"42","status":%q}`, status)
}

func refundResource(rel string) string {
	return fmt.Sprintf(`{"id":"REF1","links":[{"href":"https://api.paypal.com/v2/payments/captures/CAP1","rel":%q}]}`, rel)
}

func disputeResource(transactions string) string {
	return fmt.Sprintf(`{"dispute_id":"PP-D-1","disputed_transactions":%s}`, transactions)
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func pastWindowRFC3339() string {
	return time.Now().Add(-30 * time.Minute).UTC().Format(time.RFC3339)
}

func TestPaypalWebhook_UnconfiguredReturns503(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		verifier  *fakeVerifier
		webhookID string
	}{
		{name: "nil verifier", verifier: nil, webhookID: testPaypalWebhookID},
		{name: "empty webhook id", verifier: &fakeVerifier{verified: true}, webhookID: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := &webhookStub{}
			var v paypalVerifier
			if tt.verifier != nil {
				v = tt.verifier
			}
			h := newPaypalHandler(svc, v, tt.webhookID)

			rr := httptest.NewRecorder()
			h.paypalWebhook(rr, paypalRequest("{}"))

			if rr.Code != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want 503", rr.Code)
			}
			if tt.verifier != nil && tt.verifier.calls != 0 {
				t.Errorf("verifier calls = %d, want 0 when unconfigured", tt.verifier.calls)
			}
		})
	}
}

func TestPaypalWebhook_VerifierErrorRetries(t *testing.T) {
	t.Parallel()

	v := &fakeVerifier{err: errors.New("paypal unreachable")}
	svc := &webhookStub{}
	h := newPaypalHandler(svc, v, testPaypalWebhookID)

	rr := httptest.NewRecorder()
	h.paypalWebhook(rr, paypalRequest(paypalEnvelope("CHECKOUT.ORDER.APPROVED", nowRFC3339(), approvedResource("42"))))

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 on a verifier error", rr.Code)
	}
}

func TestPaypalWebhook_UnverifiedRejected(t *testing.T) {
	t.Parallel()

	v := &fakeVerifier{verified: false}
	svc := &webhookStub{}
	h := newPaypalHandler(svc, v, testPaypalWebhookID)

	rr := httptest.NewRecorder()
	h.paypalWebhook(rr, paypalRequest(paypalEnvelope("CHECKOUT.ORDER.APPROVED", nowRFC3339(), approvedResource("42"))))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	if len(svc.captured)+len(svc.completed)+len(svc.refunded)+len(svc.disputed)+len(svc.failed) != 0 {
		t.Errorf("no service call expected on an unverified signature")
	}
}

func TestPaypalWebhook_ApprovedCaptures(t *testing.T) {
	t.Parallel()

	svc := &webhookStub{}
	h := newPaypalHandler(svc, &fakeVerifier{verified: true}, testPaypalWebhookID)

	rr := httptest.NewRecorder()
	h.paypalWebhook(rr, paypalRequest(paypalEnvelope("CHECKOUT.ORDER.APPROVED", nowRFC3339(), approvedResource("42"))))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if len(svc.captured) != 1 || svc.captured[0].provider != "paypal" || svc.captured[0].reference != "ORDER1" || svc.captured[0].purchaseID != 42 {
		t.Errorf("captured = %+v, want one call for paypal ORDER1 purchase 42", svc.captured)
	}
}

func TestPaypalWebhook_ApprovedWithoutCustomIDStillCaptures(t *testing.T) {
	t.Parallel()

	svc := &webhookStub{}
	h := newPaypalHandler(svc, &fakeVerifier{verified: true}, testPaypalWebhookID)

	rr := httptest.NewRecorder()
	h.paypalWebhook(rr, paypalRequest(paypalEnvelope("CHECKOUT.ORDER.APPROVED", nowRFC3339(), approvedResource(""))))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if len(svc.captured) != 1 || svc.captured[0].reference != "ORDER1" || svc.captured[0].purchaseID != 0 {
		t.Errorf("captured = %+v, want one call for ORDER1 with purchase 0", svc.captured)
	}
}

func TestPaypalWebhook_ApprovedCaptureTransientErrorRetries(t *testing.T) {
	t.Parallel()

	svc := &webhookStub{captureErr: errors.New("db unavailable")}
	h := newPaypalHandler(svc, &fakeVerifier{verified: true}, testPaypalWebhookID)

	rr := httptest.NewRecorder()
	h.paypalWebhook(rr, paypalRequest(paypalEnvelope("CHECKOUT.ORDER.APPROVED", nowRFC3339(), approvedResource("42"))))

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 on a transient capture error", rr.Code)
	}
}

func TestPaypalWebhook_CaptureCompletedCredits(t *testing.T) {
	t.Parallel()

	svc := &webhookStub{}
	h := newPaypalHandler(svc, &fakeVerifier{verified: true}, testPaypalWebhookID)

	rr := httptest.NewRecorder()
	h.paypalWebhook(rr, paypalRequest(paypalEnvelope("PAYMENT.CAPTURE.COMPLETED", nowRFC3339(), captureResource("COMPLETED"))))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if len(svc.completed) != 1 || svc.completed[0].purchaseID != 42 || svc.completed[0].paymentID != "CAP1" {
		t.Errorf("completed = %+v, want one call for purchase 42 with CAP1", svc.completed)
	}
}

func TestPaypalWebhook_CaptureDeniedFails(t *testing.T) {
	t.Parallel()

	svc := &webhookStub{}
	h := newPaypalHandler(svc, &fakeVerifier{verified: true}, testPaypalWebhookID)

	rr := httptest.NewRecorder()
	h.paypalWebhook(rr, paypalRequest(paypalEnvelope("PAYMENT.CAPTURE.DENIED", nowRFC3339(), captureResource("DECLINED"))))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if len(svc.failed) != 1 || svc.failed[0] != 42 {
		t.Errorf("failed = %+v, want one call for purchase 42", svc.failed)
	}
}

func TestPaypalWebhook_RefundMarksRefunded(t *testing.T) {
	t.Parallel()

	svc := &webhookStub{}
	h := newPaypalHandler(svc, &fakeVerifier{verified: true}, testPaypalWebhookID)

	rr := httptest.NewRecorder()
	h.paypalWebhook(rr, paypalRequest(paypalEnvelope("PAYMENT.CAPTURE.REFUNDED", nowRFC3339(), refundResource("up"))))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if len(svc.refunded) != 1 || svc.refunded[0].provider != "paypal" || svc.refunded[0].paymentID != "CAP1" {
		t.Errorf("refunded = %+v, want one call for paypal CAP1", svc.refunded)
	}
}

func TestPaypalWebhook_RefundWithoutUpLinkRetries(t *testing.T) {
	t.Parallel()

	svc := &webhookStub{}
	h := newPaypalHandler(svc, &fakeVerifier{verified: true}, testPaypalWebhookID)

	rr := httptest.NewRecorder()
	h.paypalWebhook(rr, paypalRequest(paypalEnvelope("PAYMENT.CAPTURE.REFUNDED", nowRFC3339(), refundResource("self"))))

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 when the refund has no up link", rr.Code)
	}
	if len(svc.refunded) != 0 {
		t.Errorf("refunded = %+v, want none when the capture link is unresolvable", svc.refunded)
	}
}

func TestPaypalWebhook_DisputeBans(t *testing.T) {
	t.Parallel()

	svc := &webhookStub{}
	h := newPaypalHandler(svc, &fakeVerifier{verified: true}, testPaypalWebhookID)

	rr := httptest.NewRecorder()
	h.paypalWebhook(rr, paypalRequest(paypalEnvelope("CUSTOMER.DISPUTE.CREATED", nowRFC3339(), disputeResource(`[{"seller_transaction_id":"CAP1"}]`))))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if len(svc.disputed) != 1 || svc.disputed[0].provider != "paypal" || svc.disputed[0].paymentID != "CAP1" {
		t.Errorf("disputed = %+v, want one call for paypal CAP1", svc.disputed)
	}
}

func TestPaypalWebhook_DisputeScansAllTransactions(t *testing.T) {
	t.Parallel()

	svc := &webhookStub{}
	h := newPaypalHandler(svc, &fakeVerifier{verified: true}, testPaypalWebhookID)

	transactions := `[{"seller_transaction_id":""},{"seller_transaction_id":"CAP1"}]`
	rr := httptest.NewRecorder()
	h.paypalWebhook(rr, paypalRequest(paypalEnvelope("CUSTOMER.DISPUTE.CREATED", nowRFC3339(), disputeResource(transactions))))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if len(svc.disputed) != 1 || svc.disputed[0].paymentID != "CAP1" {
		t.Errorf("disputed = %+v, want one call for CAP1 from the second transaction", svc.disputed)
	}
}

func TestPaypalWebhook_DisputeWithoutCaptureIDRetries(t *testing.T) {
	t.Parallel()

	svc := &webhookStub{}
	h := newPaypalHandler(svc, &fakeVerifier{verified: true}, testPaypalWebhookID)

	rr := httptest.NewRecorder()
	h.paypalWebhook(rr, paypalRequest(paypalEnvelope("CUSTOMER.DISPUTE.CREATED", nowRFC3339(), disputeResource(`[{"seller_transaction_id":""}]`))))

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 when no capture id is present", rr.Code)
	}
	if len(svc.disputed) != 0 {
		t.Errorf("disputed = %+v, want none when no usable capture id exists", svc.disputed)
	}
}

func TestPaypalWebhook_MalformedEnvelopeAcked(t *testing.T) {
	t.Parallel()

	svc := &webhookStub{}
	h := newPaypalHandler(svc, &fakeVerifier{verified: true}, testPaypalWebhookID)

	rr := httptest.NewRecorder()
	h.paypalWebhook(rr, paypalRequest("{"))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 so PayPal does not retry a malformed envelope", rr.Code)
	}
	if len(svc.captured)+len(svc.completed)+len(svc.refunded)+len(svc.disputed)+len(svc.failed) != 0 {
		t.Errorf("no service call expected for a malformed envelope")
	}
}

func TestPaypalWebhook_UnhandledEventNoop(t *testing.T) {
	t.Parallel()

	svc := &webhookStub{}
	h := newPaypalHandler(svc, &fakeVerifier{verified: true}, testPaypalWebhookID)

	rr := httptest.NewRecorder()
	h.paypalWebhook(rr, paypalRequest(paypalEnvelope("BILLING.SUBSCRIPTION.CREATED", nowRFC3339(), "{}")))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if len(svc.captured)+len(svc.completed)+len(svc.refunded)+len(svc.disputed)+len(svc.failed) != 0 {
		t.Errorf("no service call expected for an unhandled event type")
	}
}

func TestPaypalWebhook_RefundNotFoundWithinWindowRetries(t *testing.T) {
	t.Parallel()

	svc := &webhookStub{refundErr: domain.ErrPurchaseNotFound}
	h := newPaypalHandler(svc, &fakeVerifier{verified: true}, testPaypalWebhookID)

	rr := httptest.NewRecorder()
	h.paypalWebhook(rr, paypalRequest(paypalEnvelope("PAYMENT.CAPTURE.REFUNDED", nowRFC3339(), refundResource("up"))))

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 so PayPal retries until completion lands", rr.Code)
	}
}

func TestPaypalWebhook_RefundNotFoundPastWindowGivesUp(t *testing.T) {
	t.Parallel()

	svc := &webhookStub{refundErr: domain.ErrPurchaseNotFound}
	h := newPaypalHandler(svc, &fakeVerifier{verified: true}, testPaypalWebhookID)

	rr := httptest.NewRecorder()
	h.paypalWebhook(rr, paypalRequest(paypalEnvelope("PAYMENT.CAPTURE.REFUNDED", pastWindowRFC3339(), refundResource("up"))))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 after the retry window elapsed", rr.Code)
	}
}

func TestPaypalWebhook_CompletionTransientErrorRetries(t *testing.T) {
	t.Parallel()

	svc := &webhookStub{completeErr: errors.New("db down")}
	h := newPaypalHandler(svc, &fakeVerifier{verified: true}, testPaypalWebhookID)

	rr := httptest.NewRecorder()
	h.paypalWebhook(rr, paypalRequest(paypalEnvelope("PAYMENT.CAPTURE.COMPLETED", nowRFC3339(), captureResource("COMPLETED"))))

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 on a transient fulfillment error", rr.Code)
	}
}
