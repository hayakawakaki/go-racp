package transport

import (
	"errors"
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/account/domain"
	"github.com/hayakawakaki/go-racp/internal/actiontoken"
	"github.com/hayakawakaki/go-racp/internal/httpx"
)

const maxResetPasswordFormBytes = 2 << 10

func (h *Handler) showResetPassword(w http.ResponseWriter, r *http.Request) {
	expired := resetResultPage(h.layout(), ResetResultState{Kind: ResetResultExpired})
	token, ok := h.validateTokenLink(w, r, h.svc.PeekPasswordReset, "reset_password peek", expired)
	if !ok {
		return
	}

	httpx.RenderHTML(w, r, h.logger, resetPasswordPage(h.layout(), ResetPasswordState{Token: token}))
}

func (h *Handler) doResetPassword(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Referrer-Policy", "no-referrer")
	r.Body = http.MaxBytesReader(w, r.Body, maxResetPasswordFormBytes)
	if err := r.ParseForm(); err != nil {
		httpx.RenderHTML(w, r, h.logger, resetResultPage(h.layout(), ResetResultState{Kind: ResetResultInvalid}))
		return
	}

	token := r.PostFormValue(fieldToken)
	password := r.PostFormValue(fieldPassword)
	confirm := r.PostFormValue(fieldPasswordConfirm)
	if password != confirm {
		httpx.RenderHTML(w, r, h.logger, resetPasswordPage(h.layout(), ResetPasswordState{
			Token:  token,
			Errors: map[string]string{fieldPasswordConfirm: "passwords do not match"},
		}))
		return
	}

	err := h.svc.ConsumePasswordReset(r.Context(), token, password)
	if err != nil {
		var ve *domain.ValidationError
		if errors.As(err, &ve) {
			httpx.RenderHTML(w, r, h.logger, resetPasswordPage(h.layout(), ResetPasswordState{
				Token:  token,
				Errors: ve.Fields,
			}))
			return
		}

		state := ResetResultState{Kind: ResetResultInvalid}
		switch {
		case errors.Is(err, actiontoken.ErrTokenExpired):
			state.Kind = ResetResultExpired
		case errors.Is(err, actiontoken.ErrTokenAlreadyUsed):
			state.Kind = ResetResultAlreadyUsed
		case errors.Is(err, actiontoken.ErrTokenInvalid):
			state.Kind = ResetResultInvalid
		default:
			h.logger.Error("reset_password consume", "err", err)
			state.Kind = ResetResultInvalid
		}
		httpx.RenderHTML(w, r, h.logger, resetResultPage(h.layout(), state))
		return
	}

	httpx.RenderHTML(w, r, h.logger, resetResultPage(h.layout(), ResetResultState{Kind: ResetResultSuccess}))
}
