package self

import (
	"errors"
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
	"github.com/hayakawakaki/go-racp/internal/features/account/transport/middleware"
	selfstate "github.com/hayakawakaki/go-racp/internal/features/account/transport/self/state"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
)

func (h *Handler) showChangePassword(w http.ResponseWriter, r *http.Request) {
	h.renderChangePassword(w, r, selfstate.ChangePasswordState{}, true)
}

//nolint:cyclop // splitting would obscure the flow
func (h *Handler) doChangePassword(w http.ResponseWriter, r *http.Request) {
	sess, ok := middleware.SessionFromContext(r.Context())
	if !ok || sess == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := httpx.ParseForm(w, r, maxAccountFormBytes); err != nil {
		h.renderChangePassword(w, r, selfstate.ChangePasswordState{
			Errors: map[string]string{fieldNewPassword: invalidFormDataMsg},
		}, false)
		return
	}

	cookie, err := r.Cookie(middleware.SessionCookieName)
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
		var ve *domain.ValidationError
		if errors.As(err, &ve) {
			h.renderChangePassword(w, r, selfstate.ChangePasswordState{Errors: ve.Fields}, false)
			return
		}
		if errors.Is(err, domain.ErrPasswordRecentlyChanged) {
			h.renderChangePassword(w, r, selfstate.ChangePasswordState{
				Errors: map[string]string{fieldCurrentPassword: "Password was changed recently. Please try again later."},
			}, false)
			return
		}
		h.logger.Error("update password", "err", err)
		http.Error(w, genericErrorMessage, http.StatusInternalServerError)
		return
	}

	httpx.Redirect(w, r, "/account?notice="+noticePasswordChanged)
}

// renderChangePassword renders the modal/form for HTMX requests and the full page for direct navigation.
// modalOnInitial selects between the modal wrapper (for initial GET) and the bare form (for re-renders after POST).
func (h *Handler) renderChangePassword(w http.ResponseWriter, r *http.Request, state selfstate.ChangePasswordState, modalOnInitial bool) {
	if httpx.IsHTMX(r) {
		if modalOnInitial {
			httpx.RenderHTML(w, r, h.logger, h.theme.AccountChangePasswordModal(state))
			return
		}
		httpx.RenderHTML(w, r, h.logger, h.theme.AccountChangePasswordForm(state))
		return
	}
	httpx.RenderHTML(w, r, h.logger, h.theme.AccountChangePasswordPage(h.layout(), state))
}
