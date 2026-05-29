package self

import (
	"context"
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/features/account/transport/middleware"
	selfstate "github.com/hayakawakaki/go-racp/internal/features/account/transport/self/state"
	charapp "github.com/hayakawakaki/go-racp/internal/features/character/app"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
)

const (
	noticePasswordChanged     = "password_changed"
	noticeEmailChangeSent     = "email_change_sent"
	noticeEmailChangeCooldown = "email_change_cooldown"
	noticeEmailChangeLocked   = "email_change_locked"
	noticeEmailChanged        = "email_changed"
)

var accountNoticeText = map[string]string{
	noticePasswordChanged:     "Password updated.",
	noticeEmailChangeSent:     "We've sent a confirmation link to your new email address. Click it to complete the change.",
	noticeEmailChangeCooldown: "We sent a confirmation link recently. Please check your inbox before requesting another.",
	noticeEmailChangeLocked:   "Email was changed recently. You can change it again after the cooldown expires.",
	noticeEmailChanged:        "Email updated.",

	noticeWithdrawOK:           "Withdrawal requested. It will be delivered in-game shortly.",
	noticeWithdrawLocked:       "Withdrawals are on cooldown after a recent deposit. Please wait.",
	noticeWithdrawInsufficient: "You do not have enough balance for that withdrawal.",
	noticeWithdrawInvalid:      "Invalid withdrawal amount.",
}

var characterNoticeText = map[string]string{
	"not_found": "Character not found.",
}

func (h *Handler) showAccount(w http.ResponseWriter, r *http.Request) {
	sess, ok := middleware.SessionFromContext(r.Context())
	if !ok || sess == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	account, err := h.svc.GetAccount(r.Context(), sess.UserID)
	if err != nil {
		h.logger.Error("account get", "err", err)
		http.Error(w, genericErrorMessage, http.StatusInternalServerError)
		return
	}

	var chars []charapp.CharacterDTO
	if h.characters != nil {
		chars, err = h.characters.List(r.Context(), sess.UserID)
		if err != nil {
			h.logger.Error("account characters", "err", err)
			chars = nil
		}
	}

	state := selfstate.AccountState{Account: account, Characters: chars}
	h.populateWallet(r.Context(), sess.UserID, &state)
	noticeParam := r.URL.Query().Get("notice")
	if notice, ok := accountNoticeText[noticeParam]; ok {
		state.Notice = notice
	}
	if charNotice, ok := characterNoticeText[noticeParam]; ok {
		state.Notice = charNotice
	}

	if noticeParam == middleware.NoticeBanBlocked && account.Restricted {
		state.BanBlocked = "Account changes are disabled while restricted."
	}

	httpx.RenderHTML(w, r, h.logger, h.theme.AccountPage(h.layout(), state))
}

func (h *Handler) populateWallet(ctx context.Context, userID int, state *selfstate.AccountState) {
	if h.currency == nil {
		return
	}

	balance, err := h.currency.Balance(ctx, userID)
	if err != nil {
		h.logger.Error("account balance", "err", err)
		state.BalanceFailed = true
	} else {
		state.Balance = balance
	}

	recentWithdraws, err := h.currency.RecentWithdraws(ctx, userID, 5)
	if err != nil {
		h.logger.Error("account recent withdraws", "err", err)
		state.WithdrawsFailed = true
	} else {
		state.RecentWithdraws = recentWithdraws
	}
}
