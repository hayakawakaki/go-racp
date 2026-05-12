package transport

import (
	"errors"
	"net/http"

	accountapp "github.com/hayakawakaki/go-racp/internal/account/app"
	authdomain "github.com/hayakawakaki/go-racp/internal/auth/domain"
	authtransport "github.com/hayakawakaki/go-racp/internal/auth/transport"
	"github.com/hayakawakaki/go-racp/internal/httpx"
)

func (h *Handler) showChangeEmail(w http.ResponseWriter, r *http.Request) {
	if httpx.IsHTMX(r) {
		httpx.RenderHTML(w, r, h.logger, changeEmailModal(ChangeEmailState{}))
		return
	}
	httpx.RenderHTML(w, r, h.logger, changeEmailPage(h.layout(), ChangeEmailState{}))
}

//nolint:cyclop // sequential session/form/service/validation branches; splitting would obscure the flow
func (h *Handler) doChangeEmail(w http.ResponseWriter, r *http.Request) {
	sess, ok := authtransport.SessionFromContext(r.Context())
	if !ok || sess == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxAccountFormBytes)
	if err := r.ParseForm(); err != nil {
		httpx.RenderHTML(w, r, h.logger, changeEmailForm(ChangeEmailState{
			Errors: map[string]string{"new_email": "Invalid form data."},
		}))
		return
	}
	newEmail := r.PostFormValue("new_email")
	err := h.svc.RequestEmailChange(r.Context(), sess.UserID, r.PostFormValue("current_password"), newEmail)
	if err != nil {
		if errors.Is(err, accountapp.ErrEmailChangeCooldown) {
			h.redirectWithNotice(w, r, "email_change_cooldown")
			return
		}
		if errors.Is(err, authdomain.ErrEmailRecentlyChanged) {
			h.redirectWithNotice(w, r, "email_change_locked")
			return
		}
		var ve *authdomain.ValidationError
		if errors.As(err, &ve) {
			httpx.RenderHTML(w, r, h.logger, changeEmailForm(ChangeEmailState{
				NewEmail: newEmail,
				Errors:   ve.Fields,
			}))
			return
		}
		h.logger.Error("request email change", "err", err)
		http.Error(w, genericErrorMessage, http.StatusInternalServerError)
		return
	}
	h.redirectWithNotice(w, r, "email_change_sent")
}

func (h *Handler) redirectWithNotice(w http.ResponseWriter, r *http.Request, notice string) {
	target := "/account?notice=" + notice
	if httpx.IsHTMX(r) {
		w.Header().Set("HX-Redirect", target)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}
