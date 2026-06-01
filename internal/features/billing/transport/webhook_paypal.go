package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/billing/domain"
	"github.com/hayakawakaki/go-racp/internal/features/billing/infra"
)

func (h *Handler) paypalWebhook(w http.ResponseWriter, r *http.Request) {
	if h.paypal == nil || h.paypalWebhookID == "" {
		http.Error(w, "paypal webhook not configured", http.StatusServiceUnavailable)
		return
	}

	payload, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBytes))
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	params := infra.WebhookSignatureParams{
		AuthAlgo:         r.Header.Get("Paypal-Auth-Algo"),
		CertURL:          r.Header.Get("Paypal-Cert-Url"),
		TransmissionID:   r.Header.Get("Paypal-Transmission-Id"),
		TransmissionSig:  r.Header.Get("Paypal-Transmission-Sig"),
		TransmissionTime: r.Header.Get("Paypal-Transmission-Time"),
		WebhookID:        h.paypalWebhookID,
		Event:            payload,
	}

	verified, err := h.paypal.VerifyWebhook(r.Context(), params)
	if err != nil {
		h.logger.Error("paypal webhook: signature verification failed, asking PayPal to retry", "err", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	if !verified {
		h.logger.Warn("paypal webhook: signature rejected")
		http.Error(w, "bad signature", http.StatusBadRequest)
		return
	}

	var envelope struct {
		ID         string          `json:"id"`
		EventType  string          `json:"event_type"`
		CreateTime time.Time       `json:"create_time"`
		Resource   json.RawMessage `json:"resource"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		h.logger.Error("paypal webhook: bad event envelope", "err", err)
		w.WriteHeader(http.StatusOK)
		return
	}

	if err := h.dispatchPaypalEvent(r.Context(), envelope.EventType, envelope.CreateTime, envelope.Resource); err != nil {
		h.logger.Error("paypal webhook: transient failure, asking PayPal to retry",
			"type", envelope.EventType, "event_id", envelope.ID, "err", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) dispatchPaypalEvent(ctx context.Context, eventType string, eventTime time.Time, resource json.RawMessage) error {
	switch eventType {
	case "CHECKOUT.ORDER.APPROVED":
		return h.handlePaypalApproved(ctx, resource)
	case "PAYMENT.CAPTURE.COMPLETED":
		return h.handlePaypalCaptureCompleted(ctx, resource)
	case "PAYMENT.CAPTURE.DENIED":
		return h.handlePaypalCaptureDenied(ctx, resource)
	case "PAYMENT.CAPTURE.REFUNDED":
		return h.handlePaypalCaptureRefunded(ctx, eventTime, resource)
	case "CUSTOMER.DISPUTE.CREATED":
		return h.handlePaypalDisputeCreated(ctx, eventTime, resource)
	default:
		return nil
	}
}

func (h *Handler) handlePaypalApproved(ctx context.Context, resource json.RawMessage) error {
	var order struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(resource, &order); err != nil {
		h.logger.Error("paypal webhook: bad order payload", "err", err)
		return nil
	}
	if order.ID == "" {
		h.logger.Error("paypal webhook: approved order missing id")
		return nil
	}

	_, err := h.paypal.CaptureOrder(ctx, order.ID)
	if err != nil {
		if errors.Is(err, infra.ErrPaypalOrderAlreadyCaptured) {
			return nil
		}

		return fmt.Errorf("transport.Handler.paypalWebhook: %w", err)
	}

	return nil
}

func (h *Handler) handlePaypalCaptureCompleted(ctx context.Context, resource json.RawMessage) error {
	var capture struct {
		ID       string `json:"id"`
		CustomID string `json:"custom_id"`
	}
	if err := json.Unmarshal(resource, &capture); err != nil {
		h.logger.Error("paypal webhook: bad capture payload", "err", err)
		return nil
	}

	purchaseID, ok := parsePurchaseID(capture.CustomID)
	if !ok {
		h.logger.Error("paypal webhook: missing or invalid custom_id", "capture", capture.ID)
		return nil
	}

	return h.ackPaypalFulfillment(
		h.svc.CompletePurchase(ctx, purchaseID, capture.ID),
		"completion", "capture", capture.ID, "purchase_id", purchaseID,
	)
}

func (h *Handler) handlePaypalCaptureDenied(ctx context.Context, resource json.RawMessage) error {
	var capture struct {
		ID       string `json:"id"`
		CustomID string `json:"custom_id"`
	}
	if err := json.Unmarshal(resource, &capture); err != nil {
		h.logger.Error("paypal webhook: bad capture payload", "err", err)
		return nil
	}

	purchaseID, ok := parsePurchaseID(capture.CustomID)
	if !ok {
		h.logger.Error("paypal webhook: missing or invalid custom_id", "capture", capture.ID)
		return nil
	}

	return h.ackPaypalFulfillment(
		h.svc.FailPurchase(ctx, purchaseID),
		"failed", "capture", capture.ID, "purchase_id", purchaseID,
	)
}

func (h *Handler) handlePaypalCaptureRefunded(ctx context.Context, eventTime time.Time, resource json.RawMessage) error {
	var refund struct {
		ID    string `json:"id"`
		Links []struct {
			Href string `json:"href"`
			Rel  string `json:"rel"`
		} `json:"links"`
	}
	if err := json.Unmarshal(resource, &refund); err != nil {
		h.logger.Error("paypal webhook: bad refund payload", "err", err)
		return nil
	}

	captureID := paypalCaptureIDFromLinks(refund.Links)
	if captureID == "" {
		h.logger.Error("paypal webhook: refund missing capture link", "refund", refund.ID)
		return nil
	}

	return h.ackPaypalPaymentMatch(
		h.svc.RefundPurchase(ctx, "paypal", captureID),
		eventTime, "refund", "refund", refund.ID,
	)
}

func (h *Handler) handlePaypalDisputeCreated(ctx context.Context, eventTime time.Time, resource json.RawMessage) error {
	var dispute struct {
		DisputeID            string `json:"dispute_id"`
		DisputedTransactions []struct {
			SellerTransactionID string `json:"seller_transaction_id"`
		} `json:"disputed_transactions"`
	}
	if err := json.Unmarshal(resource, &dispute); err != nil {
		h.logger.Error("paypal webhook: bad dispute payload", "err", err)
		return nil
	}
	if len(dispute.DisputedTransactions) == 0 || dispute.DisputedTransactions[0].SellerTransactionID == "" {
		h.logger.Error("paypal webhook: dispute missing capture id", "dispute", dispute.DisputeID)
		return nil
	}

	captureID := dispute.DisputedTransactions[0].SellerTransactionID

	return h.ackPaypalPaymentMatch(
		h.svc.DisputePurchase(ctx, "paypal", captureID),
		eventTime, "dispute", "dispute", dispute.DisputeID,
	)
}

func (h *Handler) ackPaypalFulfillment(err error, kind string, attrs ...any) error {
	if err == nil {
		return nil
	}

	if isTerminalFulfillmentError(err) {
		h.logger.Error("paypal webhook: fulfillment rejected, not retrying", logArgs(kind, attrs, err)...)
		return nil
	}

	return fmt.Errorf("transport.Handler.paypalWebhook: %w", err)
}

func (h *Handler) ackPaypalPaymentMatch(err error, eventTime time.Time, kind string, attrs ...any) error {
	if errors.Is(err, domain.ErrPurchaseNotFound) {
		if time.Since(eventTime) < webhookRetryWindow {
			return fmt.Errorf("transport.Handler.paypalWebhook: %w", err)
		}
		h.logger.Error("paypal webhook: purchase unmatched past retry window, giving up", logArgs(kind, attrs, err)...)
		return nil
	}

	return h.ackPaypalFulfillment(err, kind, attrs...)
}

func paypalCaptureIDFromLinks(links []struct {
	Href string `json:"href"`
	Rel  string `json:"rel"`
}) string {
	for _, link := range links {
		if link.Rel != "up" {
			continue
		}

		parsed, err := url.Parse(link.Href)
		if err != nil {
			return ""
		}

		return path.Base(parsed.Path)
	}

	return ""
}

func parsePurchaseID(raw string) (int64, bool) {
	if raw == "" {
		return 0, false
	}

	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, false
	}

	return id, true
}
