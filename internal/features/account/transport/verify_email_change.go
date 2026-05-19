package transport

import (
	"errors"
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
	actiontokendomain "github.com/hayakawakaki/go-racp/internal/platform/actiontoken/domain"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
)

func (h *Handler) showVerifyEmailChange(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Referrer-Policy", "no-referrer")

	token := r.URL.Query().Get("token")
	if token == "" {
		httpx.RenderHTML(w, r, h.logger, emailChangeResultPage(h.layout(), EmailChangeResultState{Kind: EmailChangeResultInvalid}))
		return
	}

	peeked, err := h.svc.Peek(r.Context(), actiontokendomain.EmailChange, token)
	if err != nil {
		state := EmailChangeResultState{Kind: EmailChangeResultInvalid}
		switch {
		case errors.Is(err, actiontokendomain.ErrTokenExpired):
			state.Kind = EmailChangeResultExpired
		case errors.Is(err, actiontokendomain.ErrTokenAlreadyUsed):
			state.Kind = EmailChangeResultAlready
		case errors.Is(err, actiontokendomain.ErrTokenInvalid):
			state.Kind = EmailChangeResultInvalid
		default:
			h.logger.Error("verify email change peek", "err", err)
			state.Kind = EmailChangeResultInvalid
		}
		httpx.RenderHTML(w, r, h.logger, emailChangeResultPage(h.layout(), state))
		return
	}

	httpx.RenderHTML(w, r, h.logger, verifyEmailChangeConfirmPage(h.layout(), VerifyEmailChangeConfirmState{
		Token:    token,
		NewEmail: string(peeked.Payload),
	}))
}

func (h *Handler) doVerifyEmailChange(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Referrer-Policy", "no-referrer")
	if err := httpx.ParseForm(w, r, maxVerifyFormBytes); err != nil {
		httpx.RenderHTML(w, r, h.logger, emailChangeResultPage(h.layout(), EmailChangeResultState{Kind: EmailChangeResultInvalid}))
		return
	}

	token := r.PostFormValue(fieldToken)
	if token == "" {
		httpx.RenderHTML(w, r, h.logger, emailChangeResultPage(h.layout(), EmailChangeResultState{Kind: EmailChangeResultInvalid}))
		return
	}

	user, err := h.svc.ConsumeEmailChange(r.Context(), token)
	if err != nil {
		state := EmailChangeResultState{Kind: EmailChangeResultInvalid}
		switch {
		case errors.Is(err, actiontokendomain.ErrTokenExpired):
			state.Kind = EmailChangeResultExpired
		case errors.Is(err, actiontokendomain.ErrTokenAlreadyUsed):
			state.Kind = EmailChangeResultAlready
		case errors.Is(err, actiontokendomain.ErrTokenInvalid):
			state.Kind = EmailChangeResultInvalid
		case errors.Is(err, domain.ErrEmailTaken):
			state.Kind = EmailChangeResultTaken
		case errors.Is(err, domain.ErrAccountPermaBanned),
			errors.Is(err, domain.ErrAccountTempBanned),
			errors.Is(err, domain.ErrAccountDeleted):
			state.Kind = EmailChangeResultAccountRestricted
		default:
			h.logger.Error("verify email change", "err", err)
			state.Kind = EmailChangeResultInvalid
		}
		httpx.RenderHTML(w, r, h.logger, emailChangeResultPage(h.layout(), state))
		return
	}

	httpx.RenderHTML(w, r, h.logger, emailChangeResultPage(h.layout(), EmailChangeResultState{
		Kind:     EmailChangeResultSuccess,
		NewEmail: user.Email,
	}))
}
