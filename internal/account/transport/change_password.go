package transport

import (
	"errors"
	"net/http"

	authdomain "github.com/hayakawakaki/go-racp/internal/auth/domain"
	authtransport "github.com/hayakawakaki/go-racp/internal/auth/transport"
	"github.com/hayakawakaki/go-racp/internal/httpx"
)

func (h *Handler) showChangePassword(w http.ResponseWriter, r *http.Request) {
	if httpx.IsHTMX(r) {
		httpx.RenderHTML(w, r, h.logger, changePasswordModal(ChangePasswordState{}))
		return
	}
	httpx.RenderHTML(w, r, h.logger, changePasswordPage(h.layout(), ChangePasswordState{}))
}

//nolint:cyclop // sequential session/form/cookie/service branches; splitting would obscure the flow
func (h *Handler) doChangePassword(w http.ResponseWriter, r *http.Request) {
	sess, ok := authtransport.SessionFromContext(r.Context())
	if !ok || sess == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxAccountFormBytes)
	if err := r.ParseForm(); err != nil {
		//nolint:gosec // G101 false-positive: form-error message, not a credential
		httpx.RenderHTML(w, r, h.logger, changePasswordForm(ChangePasswordState{
			Errors: map[string]string{"new_password": "Invalid form data."},
		}))
		return
	}
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	err = h.svc.UpdatePassword(r.Context(), sess.UserID, cookie.Value,
		r.PostFormValue("current_password"),
		r.PostFormValue("new_password"),
		r.PostFormValue("new_password_confirm"),
	)
	if err != nil {
		var ve *authdomain.ValidationError
		if errors.As(err, &ve) {
			httpx.RenderHTML(w, r, h.logger, changePasswordForm(ChangePasswordState{Errors: ve.Fields}))
			return
		}
		if errors.Is(err, authdomain.ErrPasswordRecentlyChanged) {
			httpx.RenderHTML(w, r, h.logger, changePasswordForm(ChangePasswordState{
				Errors: map[string]string{"current_password": "Password was changed recently. Please try again later."},
			}))
			return
		}
		h.logger.Error("update password", "err", err)
		http.Error(w, genericErrorMessage, http.StatusInternalServerError)
		return
	}
	if httpx.IsHTMX(r) {
		w.Header().Set("HX-Redirect", "/account?notice=password_changed")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, "/account?notice=password_changed", http.StatusSeeOther)
}
