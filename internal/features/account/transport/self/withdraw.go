package self

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
	"github.com/hayakawakaki/go-racp/internal/features/account/transport/middleware"
	selfstate "github.com/hayakawakaki/go-racp/internal/features/account/transport/self/state"
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

	withdrawHistoryLimit = 10

	withdrawToastTrigger = `{"toast":{"type":"success","message":"Withdrawal requested. It will be delivered in-game shortly."}}`
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
		h.withdrawError(w, r, noticeWithdrawInvalid)
		return
	}

	zeny, errZeny := parseAmount64(r.PostFormValue(fieldWithdrawZeny))
	cashpoint, errCash := parseAmount(r.PostFormValue(fieldWithdrawCashpoint))
	if errZeny != nil || errCash != nil {
		h.withdrawError(w, r, noticeWithdrawInvalid)
		return
	}

	err := h.currency.RequestWithdraw(r.Context(), sess.UserID, zeny, cashpoint)
	if err != nil {
		h.withdrawError(w, r, h.withdrawNotice(err))
		return
	}

	h.withdrawSuccess(w, r, sess.UserID)
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

func (h *Handler) showWithdraw(w http.ResponseWriter, r *http.Request) {
	sess, ok := middleware.SessionFromContext(r.Context())
	if !ok || sess == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	state := h.withdrawState(r.Context(), sess.UserID)

	h.renderWithdraw(w, r, state, true)
}

func (h *Handler) withdrawState(ctx context.Context, userID int) selfstate.WithdrawState {
	var state selfstate.WithdrawState
	if h.currency == nil {
		return state
	}

	balance, err := h.currency.Balance(ctx, userID)
	if err != nil {
		h.logger.Error("withdraw balance", "err", err)
		state.BalanceFailed = true
		return state
	}

	state.Balance = balance

	return state
}

func (h *Handler) withdrawError(w http.ResponseWriter, r *http.Request, notice string) {
	if !httpx.IsHTMX(r) {
		httpx.Redirect(w, r, "/account?notice="+notice)
		return
	}

	h.renderWithdraw(w, r, selfstate.WithdrawState{FormError: accountNoticeText[notice]}, false)
}

func (h *Handler) withdrawSuccess(w http.ResponseWriter, r *http.Request, userID int) {
	if !httpx.IsHTMX(r) {
		httpx.Redirect(w, r, "/account?notice="+noticeWithdrawOK)
		return
	}

	state := h.withdrawState(r.Context(), userID)
	w.Header().Set("HX-Trigger", withdrawToastTrigger)

	httpx.RenderHTML(w, r, h.logger, h.theme.AccountWithdrawSuccess(state))
}

func (h *Handler) renderWithdraw(w http.ResponseWriter, r *http.Request, state selfstate.WithdrawState, modalOnInitial bool) {
	if httpx.IsHTMX(r) {
		if modalOnInitial {
			httpx.RenderHTML(w, r, h.logger, h.theme.AccountWithdrawModal(state))
			return
		}
		httpx.RenderHTML(w, r, h.logger, h.theme.AccountWithdrawForm(state))
		return
	}
	httpx.RenderHTML(w, r, h.logger, h.theme.AccountWithdrawPage(h.layout(), state))
}

func (h *Handler) showWithdrawHistory(w http.ResponseWriter, r *http.Request) {
	sess, ok := middleware.SessionFromContext(r.Context())
	if !ok || sess == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	state := selfstate.WithdrawHistoryState{Location: h.general.Location()}
	if h.currency != nil {
		page, err := h.currency.WithdrawHistoryByAccount(r.Context(), sess.UserID, 1, withdrawHistoryLimit)
		if err != nil {
			h.logger.Error("account withdraw history", "err", err)
			state.Failed = true
		} else {
			state.Rows = page.Rows
		}
	}

	httpx.RenderHTML(w, r, h.logger, h.theme.AccountWithdrawHistory(state))
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
