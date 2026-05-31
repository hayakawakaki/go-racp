package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/billing/domain"
	"github.com/stripe/stripe-go/v85"
	"github.com/stripe/stripe-go/v85/webhook"
)

const (
	maxWebhookBytes    = 1 << 18
	webhookRetryWindow = 15 * time.Minute
)

func (h *Handler) stripeWebhook(w http.ResponseWriter, r *http.Request) {
	if h.stripeWebhookSecret == "" {
		http.Error(w, "stripe webhook not configured", http.StatusServiceUnavailable)
		return
	}

	payload, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBytes))
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	event, err := webhook.ConstructEventWithOptions(
		payload, r.Header.Get("Stripe-Signature"), h.stripeWebhookSecret,
		webhook.ConstructEventOptions{IgnoreAPIVersionMismatch: true},
	)
	if err != nil {
		h.logger.Warn("stripe webhook: signature verification failed", "err", err)
		http.Error(w, "bad signature", http.StatusBadRequest)
		return
	}

	if event.Data == nil {
		h.logger.Error("stripe webhook: event missing data object", "type", string(event.Type), "event_id", event.ID)
		w.WriteHeader(http.StatusOK)
		return
	}

	if err := h.dispatchStripeEvent(r.Context(), event); err != nil {
		h.logger.Error("stripe webhook: transient failure, asking Stripe to retry",
			"type", string(event.Type), "event_id", event.ID, "err", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) dispatchStripeEvent(ctx context.Context, event stripe.Event) error {
	switch event.Type {
	case "checkout.session.completed", "checkout.session.async_payment_succeeded":
		return h.handleCheckoutCompleted(ctx, event)
	case "checkout.session.expired", "checkout.session.async_payment_failed":
		return h.handleCheckoutFailed(ctx, event)
	case "charge.refunded":
		return h.handleChargeRefunded(ctx, event)
	case "charge.dispute.created":
		return h.handleDisputeCreated(ctx, event)
	default:
		return nil
	}
}

func (h *Handler) handleCheckoutCompleted(ctx context.Context, event stripe.Event) error {
	var sess stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
		h.logger.Error("stripe webhook: bad session payload", "event_id", event.ID, "err", err)
		return nil
	}
	if sess.PaymentStatus != stripe.CheckoutSessionPaymentStatusPaid {
		h.logger.Info("stripe webhook: session not paid, skipping", "session", sess.ID, "status", string(sess.PaymentStatus))
		return nil
	}

	purchaseID, ok := purchaseIDFromMetadata(sess.Metadata)
	if !ok {
		h.logger.Error("stripe webhook: missing or invalid purchase_id metadata", "session", sess.ID)
		return nil
	}

	paymentIntentID := ""
	if sess.PaymentIntent != nil {
		paymentIntentID = sess.PaymentIntent.ID
	} else if sess.AmountTotal > 0 {
		h.logger.Error("stripe webhook: paid session has an amount but no payment intent",
			"session", sess.ID, "purchase_id", purchaseID, "amount_total", sess.AmountTotal)
	}

	return h.ackFulfillment(
		h.svc.CompletePurchase(ctx, purchaseID, paymentIntentID),
		"completion", "session", sess.ID, "purchase_id", purchaseID,
	)
}

func (h *Handler) handleCheckoutFailed(ctx context.Context, event stripe.Event) error {
	var sess stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
		h.logger.Error("stripe webhook: bad session payload", "event_id", event.ID, "err", err)
		return nil
	}

	purchaseID, ok := purchaseIDFromMetadata(sess.Metadata)
	if !ok {
		h.logger.Error("stripe webhook: missing or invalid purchase_id metadata", "session", sess.ID)
		return nil
	}

	return h.ackFulfillment(h.svc.FailPurchase(ctx, purchaseID), "failed", "session", sess.ID, "purchase_id", purchaseID)
}

func (h *Handler) handleChargeRefunded(ctx context.Context, event stripe.Event) error {
	var charge stripe.Charge
	if err := json.Unmarshal(event.Data.Raw, &charge); err != nil {
		h.logger.Error("stripe webhook: bad charge payload", "event_id", event.ID, "err", err)
		return nil
	}
	if charge.PaymentIntent == nil {
		h.logger.Error("stripe webhook: refund without payment intent", "charge", charge.ID)
		return nil
	}

	return h.ackPaymentMatch(h.svc.RefundPurchase(ctx, "stripe", charge.PaymentIntent.ID), event, "refund", "charge", charge.ID)
}

func (h *Handler) handleDisputeCreated(ctx context.Context, event stripe.Event) error {
	var dispute stripe.Dispute
	if err := json.Unmarshal(event.Data.Raw, &dispute); err != nil {
		h.logger.Error("stripe webhook: bad dispute payload", "event_id", event.ID, "err", err)
		return nil
	}
	if dispute.PaymentIntent == nil {
		h.logger.Error("stripe webhook: dispute without payment intent", "dispute", dispute.ID)
		return nil
	}

	return h.ackPaymentMatch(h.svc.DisputePurchase(ctx, "stripe", dispute.PaymentIntent.ID), event, "dispute", "dispute", dispute.ID)
}

func (h *Handler) ackFulfillment(err error, kind string, attrs ...any) error {
	if err == nil {
		return nil
	}

	if isTerminalFulfillmentError(err) {
		h.logger.Error("stripe webhook: fulfillment rejected, not retrying", logArgs(kind, attrs, err)...)
		return nil
	}

	return fmt.Errorf("transport.Handler.stripeWebhook: %w", err)
}

func (h *Handler) ackPaymentMatch(err error, event stripe.Event, kind string, attrs ...any) error {
	if errors.Is(err, domain.ErrPurchaseNotFound) {
		if time.Since(time.Unix(event.Created, 0)) < webhookRetryWindow {
			return fmt.Errorf("transport.Handler.stripeWebhook: %w", err)
		}
		h.logger.Error("stripe webhook: purchase unmatched past retry window, giving up", logArgs(kind, attrs, err)...)
		return nil
	}

	return h.ackFulfillment(err, kind, attrs...)
}

func logArgs(kind string, attrs []any, err error) []any {
	args := make([]any, 0, len(attrs)+4)
	args = append(args, "kind", kind)
	args = append(args, attrs...)
	args = append(args, "err", err)

	return args
}

func purchaseIDFromMetadata(metadata map[string]string) (int64, bool) {
	raw, ok := metadata["purchase_id"]
	if !ok {
		return 0, false
	}

	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, false
	}

	return id, true
}

func isTerminalFulfillmentError(err error) bool {
	return errors.Is(err, domain.ErrPurchaseNotFound) ||
		errors.Is(err, domain.ErrUnknownPackage)
}
