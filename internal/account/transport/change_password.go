package transport

import (
	"errors"
	"net/http"

	authdomain "github.com/hayakawakaki/go-racp/internal/auth/domain"
	authtransport "github.com/hayakawakaki/go-racp/internal/auth/transport"
	"github.com/hayakawakaki/go-racp/internal/httpx"
)

func (h *Handler) showChangePassword(w http.ResponseWriter, r *http.Request) {
	h.renderChangePassword(w, r, ChangePasswordState{}, true)
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
		h.renderChangePassword(w, r, ChangePasswordState{
			Errors: map[string]string{fieldNewPassword: invalidFormDataMsg},
		}, false)
		return
	}
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	err = h.svc.UpdatePassword(r.Context(), sess.UserID, cookie.Value,
		r.PostFormValue(fieldCurrentPassword),
		r.PostFormValue(fieldNewPassword),
		r.PostFormValue(fieldNewPasswordConfirm),
	)
	if err != nil {
		var ve *authdomain.ValidationError
		if errors.As(err, &ve) {
			h.renderChangePassword(w, r, ChangePasswordState{Errors: ve.Fields}, false)
			return
		}
		if errors.Is(err, authdomain.ErrPasswordRecentlyChanged) {
			h.renderChangePassword(w, r, ChangePasswordState{
				Errors: map[string]string{fieldCurrentPassword: "Password was changed recently. Please try again later."},
			}, false)
			return
		}
		h.logger.Error("update password", "err", err)
		http.Error(w, genericErrorMessage, http.StatusInternalServerError)
		return
	}
	if httpx.IsHTMX(r) {
		w.Header().Set("HX-Redirect", "/account?notice="+noticePasswordChanged)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, "/account?notice="+noticePasswordChanged, http.StatusSeeOther)
}

// renderChangePassword renders the modal/form for HTMX requests and the full page for direct navigation.
// modalOnInitial selects between the modal wrapper (for initial GET) and the bare form (for re-renders after POST).
func (h *Handler) renderChangePassword(w http.ResponseWriter, r *http.Request, state ChangePasswordState, modalOnInitial bool) {
	if httpx.IsHTMX(r) {
		if modalOnInitial {
			httpx.RenderHTML(w, r, h.logger, changePasswordModal(state))
			return
		}
		httpx.RenderHTML(w, r, h.logger, changePasswordForm(state))
		return
	}
	httpx.RenderHTML(w, r, h.logger, changePasswordPage(h.layout(), state))
}
