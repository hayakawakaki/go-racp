package self

import (
	"errors"
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
	actiontokendomain "github.com/hayakawakaki/go-racp/internal/platform/actiontoken/domain"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
)

const maxResetPasswordFormBytes = 2 << 10

func (h *Handler) showResetPassword(w http.ResponseWriter, r *http.Request) {
	expired := resetResultPage(h.layout(), ResetResultState{Kind: ResetResultExpired})
	token, ok := h.validateTokenLink(w, r, actiontokendomain.PasswordReset, "reset_password peek", expired)
	if !ok {
		return
	}

	httpx.RenderHTML(w, r, h.logger, resetPasswordPage(h.layout(), ResetPasswordState{Token: token}))
}

func (h *Handler) doResetPassword(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Referrer-Policy", "no-referrer")
	if err := httpx.ParseForm(w, r, maxResetPasswordFormBytes); err != nil {
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
		case errors.Is(err, actiontokendomain.ErrTokenExpired):
			state.Kind = ResetResultExpired
		case errors.Is(err, actiontokendomain.ErrTokenAlreadyUsed):
			state.Kind = ResetResultAlreadyUsed
		case errors.Is(err, actiontokendomain.ErrTokenInvalid):
			state.Kind = ResetResultInvalid
		case errors.Is(err, domain.ErrAccountPermaBanned),
			errors.Is(err, domain.ErrAccountTempBanned),
			errors.Is(err, domain.ErrAccountDeleted):
			state.Kind = ResetResultAccountRestricted
		default:
			h.logger.Error("reset_password consume", "err", err)
			state.Kind = ResetResultInvalid
		}
		httpx.RenderHTML(w, r, h.logger, resetResultPage(h.layout(), state))
		return
	}

	httpx.RenderHTML(w, r, h.logger, resetResultPage(h.layout(), ResetResultState{Kind: ResetResultSuccess}))
}
