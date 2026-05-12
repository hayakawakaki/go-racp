package transport

import (
	"net/http"

	authtransport "github.com/hayakawakaki/go-racp/internal/auth/transport"
	"github.com/hayakawakaki/go-racp/internal/httpx"
)

var accountNoticeText = map[string]string{
	"password_changed":      "Password updated.",
	"email_change_sent":     "We've sent a confirmation link to your new email address. Click it to complete the change.",
	"email_change_cooldown": "We sent a confirmation link recently. Please check your inbox before requesting another.",
	"email_change_locked":   "Email was changed recently. You can change it again after the cooldown expires.",
	"email_changed":         "Email updated.",
}

func (h *Handler) showAccount(w http.ResponseWriter, r *http.Request) {
	sess, ok := authtransport.SessionFromContext(r.Context())
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
	state := AccountState{Account: account}
	if notice, ok := accountNoticeText[r.URL.Query().Get("notice")]; ok {
		state.Notice = notice
	}
	httpx.RenderHTML(w, r, h.logger, accountPage(h.layout(), state))
}
