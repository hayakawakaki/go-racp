package self

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
	"github.com/hayakawakaki/go-racp/internal/features/account/transport/middleware"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
)

const (
	noticeWithdrawOK           = "withdraw_ok"
	noticeWithdrawLocked       = "withdraw_locked"
	noticeWithdrawInsufficient = "withdraw_insufficient"
	noticeWithdrawInvalid      = "withdraw_invalid"
	noticeWithdrawBridge       = "withdraw_bridge"

	fieldWithdrawZeny      = "zeny"
	fieldWithdrawCashpoint = "cashpoint"
)

func (h *Handler) doWithdraw(w http.ResponseWriter, r *http.Request) {
	sess, ok := middleware.SessionFromContext(r.Context())
	if !ok || sess == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if h.currency == nil {
		httpx.Redirect(w, r, "/account")
		return
	}

	if err := httpx.ParseForm(w, r, maxWithdrawFormBytes); err != nil {
		httpx.Redirect(w, r, "/account?notice="+noticeWithdrawInvalid)
		return
	}

	zeny, errZeny := parseAmount64(r.PostFormValue(fieldWithdrawZeny))
	cashpoint, errCash := parseAmount(r.PostFormValue(fieldWithdrawCashpoint))
	if errZeny != nil || errCash != nil {
		httpx.Redirect(w, r, "/account?notice="+noticeWithdrawInvalid)
		return
	}

	err := h.currency.RequestWithdraw(r.Context(), sess.UserID, zeny, cashpoint)
	httpx.Redirect(w, r, "/account?notice="+h.withdrawNotice(err))
}

func (h *Handler) withdrawNotice(err error) string {
	switch {
	case err == nil:
		return noticeWithdrawOK
	case errors.Is(err, domain.ErrWithdrawLocked):
		return noticeWithdrawLocked
	case errors.Is(err, domain.ErrInsufficientBalance):
		return noticeWithdrawInsufficient
	case errors.Is(err, domain.ErrInvalidAmount):
		return noticeWithdrawInvalid
	case errors.Is(err, domain.ErrBridgeUnavailable):
		return noticeWithdrawBridge
	default:
		h.logger.Error("account withdraw", "err", err)
		return noticeWithdrawInvalid
	}
}

func parseAmount(raw string) (int, error) {
	if raw == "" {
		return 0, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("self.parseAmount: %w", err)
	}

	return value, nil
}

func parseAmount64(raw string) (int64, error) {
	if raw == "" {
		return 0, nil
	}

	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("self.parseAmount64: %w", err)
	}

	return value, nil
}
