package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func (h *Handler) nowpaymentsWebhook(w http.ResponseWriter, r *http.Request) {
	if h.nowpayments == nil {
		http.Error(w, "nowpayments webhook not configured", http.StatusServiceUnavailable)
		return
	}

	payload, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBytes))
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	verified, err := h.nowpayments.VerifyIPN(r.Header.Get("x-nowpayments-sig"), payload)
	if err != nil {
		h.logger.Warn("crypto webhook: signature verification failed", "err", err)
		http.Error(w, "bad signature", http.StatusBadRequest)
		return
	}
	if !verified {
		h.logger.Warn("crypto webhook: signature rejected")
		http.Error(w, "bad signature", http.StatusBadRequest)
		return
	}

	var notification struct {
		PaymentStatus string      `json:"payment_status"`
		OrderID       string      `json:"order_id"`
		PaymentID     json.Number `json:"payment_id"`
	}
	if err := json.Unmarshal(payload, &notification); err != nil {
		h.logger.Error("crypto webhook: bad ipn payload", "err", err)
		w.WriteHeader(http.StatusOK)
		return
	}

	if err := h.dispatchNowpaymentsIPN(r.Context(), notification.PaymentStatus, notification.OrderID, notification.PaymentID.String()); err != nil {
		h.logger.Error("crypto webhook: transient failure, asking NowPayments to retry",
			"status", notification.PaymentStatus, "order_id", notification.OrderID, "err", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) dispatchNowpaymentsIPN(ctx context.Context, status, orderID, paymentID string) error {
	purchaseID, ok := parsePurchaseID(orderID)
	if !ok {
		h.logger.Error("crypto webhook: missing or invalid order_id", "order_id", orderID, "status", status)
		return nil
	}

	switch status {
	case "finished":
		return h.ackCryptoFulfillment(
			h.svc.CompletePurchase(ctx, purchaseID, paymentID),
			"completion", "payment_id", paymentID, "purchase_id", purchaseID,
		)
	case "partially_paid":
		h.logger.Warn("crypto webhook: partial payment received, marking failed without crediting",
			"purchase_id", purchaseID, "payment_id", paymentID)
		return h.ackCryptoFulfillment(
			h.svc.FailPurchase(ctx, purchaseID),
			"partial", "payment_id", paymentID, "purchase_id", purchaseID,
		)
	case "failed", "expired":
		return h.ackCryptoFulfillment(
			h.svc.FailPurchase(ctx, purchaseID),
			"failed", "payment_id", paymentID, "purchase_id", purchaseID,
		)
	case "refunded":
		return h.ackCryptoFulfillment(
			h.svc.RefundPurchase(ctx, "crypto", paymentID),
			"refund", "payment_id", paymentID, "purchase_id", purchaseID,
		)
	default:
		return nil
	}
}

func (h *Handler) ackCryptoFulfillment(err error, kind string, attrs ...any) error {
	if err == nil {
		return nil
	}

	if isTerminalFulfillmentError(err) {
		h.logger.Error("crypto webhook: fulfillment rejected, not retrying", logArgs(kind, attrs, err)...)
		return nil
	}

	return fmt.Errorf("transport.Handler.nowpaymentsWebhook: %w", err)
}
