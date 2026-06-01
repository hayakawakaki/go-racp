package transport

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hayakawakaki/go-racp/internal/features/billing/domain"
)

type nowpaymentsVerifierStub struct {
	err      error
	verified bool
}

var _ nowpaymentsVerifier = (*nowpaymentsVerifierStub)(nil)

func (s *nowpaymentsVerifierStub) VerifyIPN(string, []byte) (bool, error) {
	return s.verified, s.err
}

func newNowpaymentsWebhookHandler(svc billingService, verifier nowpaymentsVerifier) *Handler {
	return NewHandler(svc, HandlerConfig{Logger: discardLogger(), NowPayments: verifier})
}

func nowpaymentsIPNRequest(body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/webhooks/nowpayments", strings.NewReader(body))
	req.Header.Set("x-nowpayments-sig", "sig")

	return req
}

func nowpaymentsIPNBody(status, orderID, paymentID string) string {
	return fmt.Sprintf(`{"payment_status":%q,"order_id":%q,"payment_id":%s}`, status, orderID, paymentID)
}

func TestNowpaymentsWebhook_FinishedCredits(t *testing.T) {
	t.Parallel()

	svc := &webhookStub{}
	h := newNowpaymentsWebhookHandler(svc, &nowpaymentsVerifierStub{verified: true})

	rr := httptest.NewRecorder()
	h.nowpaymentsWebhook(rr, nowpaymentsIPNRequest(nowpaymentsIPNBody("finished", "42", "5667108377")))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if len(svc.completed) != 1 || svc.completed[0].purchaseID != 42 || svc.completed[0].paymentID != "5667108377" {
		t.Errorf("completed = %+v, want one call for purchase 42 with 5667108377", svc.completed)
	}
}

func TestNowpaymentsWebhook_PartiallyPaidFails(t *testing.T) {
	t.Parallel()

	svc := &webhookStub{}
	h := newNowpaymentsWebhookHandler(svc, &nowpaymentsVerifierStub{verified: true})

	rr := httptest.NewRecorder()
	h.nowpaymentsWebhook(rr, nowpaymentsIPNRequest(nowpaymentsIPNBody("partially_paid", "42", "5667108377")))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if len(svc.failed) != 1 || svc.failed[0] != 42 {
		t.Errorf("failed = %v, want [42]", svc.failed)
	}
	if len(svc.completed) != 0 {
		t.Errorf("completed = %+v, want none for a partial payment", svc.completed)
	}
}

func TestNowpaymentsWebhook_FailedAndExpiredFail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status string
	}{
		{name: "failed", status: "failed"},
		{name: "expired", status: "expired"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := &webhookStub{}
			h := newNowpaymentsWebhookHandler(svc, &nowpaymentsVerifierStub{verified: true})

			rr := httptest.NewRecorder()
			h.nowpaymentsWebhook(rr, nowpaymentsIPNRequest(nowpaymentsIPNBody(tt.status, "42", "5667108377")))

			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rr.Code)
			}
			if len(svc.failed) != 1 || svc.failed[0] != 42 {
				t.Errorf("failed = %v, want [42]", svc.failed)
			}
		})
	}
}

func TestNowpaymentsWebhook_RefundedMarksRefunded(t *testing.T) {
	t.Parallel()

	svc := &webhookStub{}
	h := newNowpaymentsWebhookHandler(svc, &nowpaymentsVerifierStub{verified: true})

	rr := httptest.NewRecorder()
	h.nowpaymentsWebhook(rr, nowpaymentsIPNRequest(nowpaymentsIPNBody("refunded", "42", "5667108377")))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if len(svc.refunded) != 1 || svc.refunded[0].provider != "crypto" || svc.refunded[0].paymentID != "5667108377" {
		t.Errorf("refunded = %+v, want one call for crypto 5667108377", svc.refunded)
	}
}

func TestNowpaymentsWebhook_IntermediateStatusNoop(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status string
	}{
		{name: "waiting", status: "waiting"},
		{name: "confirming", status: "confirming"},
		{name: "confirmed", status: "confirmed"},
		{name: "sending", status: "sending"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := &webhookStub{}
			h := newNowpaymentsWebhookHandler(svc, &nowpaymentsVerifierStub{verified: true})

			rr := httptest.NewRecorder()
			h.nowpaymentsWebhook(rr, nowpaymentsIPNRequest(nowpaymentsIPNBody(tt.status, "42", "5667108377")))

			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rr.Code)
			}
			if len(svc.completed)+len(svc.failed)+len(svc.refunded) != 0 {
				t.Errorf("no fulfiller call expected for an intermediate status")
			}
		})
	}
}

func TestNowpaymentsWebhook_SignatureRejected(t *testing.T) {
	t.Parallel()

	svc := &webhookStub{}
	h := newNowpaymentsWebhookHandler(svc, &nowpaymentsVerifierStub{verified: false})

	rr := httptest.NewRecorder()
	h.nowpaymentsWebhook(rr, nowpaymentsIPNRequest(nowpaymentsIPNBody("finished", "42", "5667108377")))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	if len(svc.completed)+len(svc.failed)+len(svc.refunded) != 0 {
		t.Errorf("no fulfiller call expected for a rejected signature")
	}
}

func TestNowpaymentsWebhook_VerifyError(t *testing.T) {
	t.Parallel()

	svc := &webhookStub{}
	h := newNowpaymentsWebhookHandler(svc, &nowpaymentsVerifierStub{err: errors.New("boom")})

	rr := httptest.NewRecorder()
	h.nowpaymentsWebhook(rr, nowpaymentsIPNRequest(nowpaymentsIPNBody("finished", "42", "5667108377")))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	if len(svc.completed)+len(svc.failed)+len(svc.refunded) != 0 {
		t.Errorf("no fulfiller call expected for a verification error")
	}
}

func TestNowpaymentsWebhook_Unconfigured(t *testing.T) {
	t.Parallel()

	h := NewHandler(&webhookStub{}, HandlerConfig{Logger: discardLogger()})

	rr := httptest.NewRecorder()
	h.nowpaymentsWebhook(rr, nowpaymentsIPNRequest(nowpaymentsIPNBody("finished", "42", "5667108377")))

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rr.Code)
	}
}

func TestNowpaymentsWebhook_MalformedBodyReturns200(t *testing.T) {
	t.Parallel()

	svc := &webhookStub{}
	h := newNowpaymentsWebhookHandler(svc, &nowpaymentsVerifierStub{verified: true})

	rr := httptest.NewRecorder()
	h.nowpaymentsWebhook(rr, nowpaymentsIPNRequest("not json"))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if len(svc.completed)+len(svc.failed)+len(svc.refunded) != 0 {
		t.Errorf("no fulfiller call expected for a malformed body")
	}
}

func TestNowpaymentsWebhook_InvalidOrderIDNoop(t *testing.T) {
	t.Parallel()

	svc := &webhookStub{}
	h := newNowpaymentsWebhookHandler(svc, &nowpaymentsVerifierStub{verified: true})

	rr := httptest.NewRecorder()
	h.nowpaymentsWebhook(rr, nowpaymentsIPNRequest(nowpaymentsIPNBody("finished", "abc", "5667108377")))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if len(svc.completed)+len(svc.failed)+len(svc.refunded) != 0 {
		t.Errorf("no fulfiller call expected for an invalid order_id")
	}
}

func TestNowpaymentsWebhook_TransientErrorRetries(t *testing.T) {
	t.Parallel()

	svc := &webhookStub{completeErr: errors.New("db down")}
	h := newNowpaymentsWebhookHandler(svc, &nowpaymentsVerifierStub{verified: true})

	rr := httptest.NewRecorder()
	h.nowpaymentsWebhook(rr, nowpaymentsIPNRequest(nowpaymentsIPNBody("finished", "42", "5667108377")))

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 on a transient fulfillment error", rr.Code)
	}
}

func TestNowpaymentsWebhook_PurchaseNotFoundTerminal(t *testing.T) {
	t.Parallel()

	svc := &webhookStub{completeErr: domain.ErrPurchaseNotFound}
	h := newNowpaymentsWebhookHandler(svc, &nowpaymentsVerifierStub{verified: true})

	rr := httptest.NewRecorder()
	h.nowpaymentsWebhook(rr, nowpaymentsIPNRequest(nowpaymentsIPNBody("finished", "42", "5667108377")))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 for a terminal fulfillment error", rr.Code)
	}
}
