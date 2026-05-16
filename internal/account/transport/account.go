package transport

import (
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/account/app"
	"github.com/hayakawakaki/go-racp/internal/account/transport/middleware"
	"github.com/hayakawakaki/go-racp/internal/httpx"
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

	state := AccountState{Account: account}
	noticeParam := r.URL.Query().Get("notice")
	if notice, ok := accountNoticeText[noticeParam]; ok {
		state.Notice = notice
	}

	if noticeParam == "ban_blocked" {
		if snap, hasSnap := middleware.SnapshotFromContext(r.Context()); hasSnap && snap.Tier == app.TierTempBanned {
			state.BanBlocked = "Account changes are disabled while restricted."
		}
	}

	httpx.RenderHTML(w, r, h.logger, accountPage(h.layout(), state))
}
