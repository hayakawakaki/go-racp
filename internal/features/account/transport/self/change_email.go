package self

import (
	"errors"
	"net/http"

	app "github.com/hayakawakaki/go-racp/internal/features/account/app/self"
	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
	"github.com/hayakawakaki/go-racp/internal/features/account/transport/middleware"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
)

func (h *Handler) showChangeEmail(w http.ResponseWriter, r *http.Request) {
	h.renderChangeEmail(w, r, ChangeEmailState{}, true)
}

//nolint:cyclop // splitting would obscure the flow
func (h *Handler) doChangeEmail(w http.ResponseWriter, r *http.Request) {
	sess, ok := middleware.SessionFromContext(r.Context())
	if !ok || sess == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := httpx.ParseForm(w, r, maxAccountFormBytes); err != nil {
		h.renderChangeEmail(w, r, ChangeEmailState{
			Errors: map[string]string{fieldNewEmail: invalidFormDataMsg},
		}, false)
		return
	}

	newEmail := r.PostFormValue(fieldNewEmail)
	err := h.svc.RequestEmailChange(r.Context(), sess.UserID, r.PostFormValue(fieldCurrentPassword), newEmail)
	if err != nil {
		if errors.Is(err, app.ErrEmailChangeCooldown) {
			h.redirectWithNotice(w, r, noticeEmailChangeCooldown)
			return
		}
		if errors.Is(err, domain.ErrEmailRecentlyChanged) {
			h.redirectWithNotice(w, r, noticeEmailChangeLocked)
			return
		}
		var ve *domain.ValidationError
		if errors.As(err, &ve) {
			h.renderChangeEmail(w, r, ChangeEmailState{
				NewEmail: newEmail,
				Errors:   ve.Fields,
			}, false)
			return
		}
		h.logger.Error("request email change", "err", err)
		http.Error(w, genericErrorMessage, http.StatusInternalServerError)
		return
	}

	h.redirectWithNotice(w, r, noticeEmailChangeSent)
}

// renderChangeEmail renders the modal/form for HTMX requests and the full page for direct navigation.
// modalOnInitial selects between the modal wrapper (for initial GET) and the bare form (for re-renders after POST).
func (h *Handler) renderChangeEmail(w http.ResponseWriter, r *http.Request, state ChangeEmailState, modalOnInitial bool) {
	if httpx.IsHTMX(r) {
		if modalOnInitial {
			httpx.RenderHTML(w, r, h.logger, changeEmailModal(state))
			return
		}
		httpx.RenderHTML(w, r, h.logger, changeEmailForm(state))
		return
	}
	httpx.RenderHTML(w, r, h.logger, changeEmailPage(h.layout(), state))
}

func (h *Handler) redirectWithNotice(w http.ResponseWriter, r *http.Request, notice string) {
	httpx.Redirect(w, r, "/account?notice="+notice)
}
