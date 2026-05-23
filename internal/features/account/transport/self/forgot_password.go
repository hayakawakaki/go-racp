package self

import (
	"errors"
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
	selfstate "github.com/hayakawakaki/go-racp/internal/features/account/transport/self/state"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
)

const maxForgotPasswordFormBytes = 1 << 10

func (h *Handler) showForgotPassword(w http.ResponseWriter, r *http.Request) {
	if h.hasActiveSession(r) {
		httpx.Redirect(w, r, "/")
		return
	}

	httpx.RenderHTML(w, r, h.logger, h.theme.AccountForgotPasswordPage(h.layout(), selfstate.ForgotPasswordState{}))
}

func (h *Handler) doForgotPassword(w http.ResponseWriter, r *http.Request) {
	if err := httpx.ParseForm(w, r, maxForgotPasswordFormBytes); err != nil {
		h.renderForgotPassword(w, r, selfstate.ForgotPasswordState{Errors: map[string]string{fieldEmail: invalidFormDataMsg}})
		return
	}

	email := r.PostFormValue(fieldEmail)
	err := h.svc.RequestPasswordReset(r.Context(), email)
	if err != nil {
		state := selfstate.ForgotPasswordState{Email: email}
		var ve *domain.ValidationError
		if errors.As(err, &ve) {
			state.Errors = ve.Fields
		} else {
			h.logger.Error("forgot_password", "err", err)
			state.Errors = map[string]string{fieldEmail: genericErrorMessage}
		}
		h.renderForgotPassword(w, r, state)
		return
	}

	h.renderForgotPassword(w, r, selfstate.ForgotPasswordState{Submitted: true})
}

func (h *Handler) renderForgotPassword(w http.ResponseWriter, r *http.Request, state selfstate.ForgotPasswordState) {
	if httpx.IsHTMX(r) {
		httpx.RenderHTML(w, r, h.logger, h.theme.AccountForgotPasswordForm(state))
		return
	}
	httpx.RenderHTML(w, r, h.logger, h.theme.AccountForgotPasswordPage(h.layout(), state))
}
