package transport

import (
	"errors"
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/account/domain"
	"github.com/hayakawakaki/go-racp/internal/httpx"
)

const maxForgotPasswordFormBytes = 1 << 10

func (h *Handler) showForgotPassword(w http.ResponseWriter, r *http.Request) {
	httpx.RenderHTML(w, r, h.logger, forgotPasswordPage(h.layout(), ForgotPasswordState{}))
}

func (h *Handler) doForgotPassword(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxForgotPasswordFormBytes)
	if err := r.ParseForm(); err != nil {
		h.renderForgotPassword(w, r, ForgotPasswordState{Errors: map[string]string{fieldEmail: invalidFormDataMsg}})
		return
	}
	email := r.PostFormValue(fieldEmail)
	err := h.svc.RequestPasswordReset(r.Context(), email)
	if err != nil {
		state := ForgotPasswordState{Email: email}
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
	h.renderForgotPassword(w, r, ForgotPasswordState{Submitted: true})
}

func (h *Handler) renderForgotPassword(w http.ResponseWriter, r *http.Request, state ForgotPasswordState) {
	if httpx.IsHTMX(r) {
		httpx.RenderHTML(w, r, h.logger, forgotPasswordForm(state))
		return
	}
	httpx.RenderHTML(w, r, h.logger, forgotPasswordPage(h.layout(), state))
}
