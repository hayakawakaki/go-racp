package self

import (
	"errors"
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
	selfstate "github.com/hayakawakaki/go-racp/internal/features/account/transport/self/state"
	actiontokendomain "github.com/hayakawakaki/go-racp/internal/platform/actiontoken/domain"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
)

func (h *Handler) showVerifyEmailChange(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Referrer-Policy", "no-referrer")

	token := r.URL.Query().Get("token")
	if token == "" {
		httpx.RenderHTML(w, r, h.logger, h.theme.AccountEmailChangeResultPage(h.layout(), selfstate.EmailChangeResultState{Kind: selfstate.EmailChangeResultInvalid}))
		return
	}

	peeked, err := h.svc.Peek(r.Context(), actiontokendomain.EmailChange, token)
	if err != nil {
		state := selfstate.EmailChangeResultState{Kind: selfstate.EmailChangeResultInvalid}
		switch {
		case errors.Is(err, actiontokendomain.ErrTokenExpired):
			state.Kind = selfstate.EmailChangeResultExpired
		case errors.Is(err, actiontokendomain.ErrTokenAlreadyUsed):
			state.Kind = selfstate.EmailChangeResultAlready
		case errors.Is(err, actiontokendomain.ErrTokenInvalid):
			state.Kind = selfstate.EmailChangeResultInvalid
		default:
			h.logger.Error("verify email change peek", "err", err)
			state.Kind = selfstate.EmailChangeResultInvalid
		}
		httpx.RenderHTML(w, r, h.logger, h.theme.AccountEmailChangeResultPage(h.layout(), state))
		return
	}

	httpx.RenderHTML(w, r, h.logger, h.theme.AccountVerifyEmailChangeConfirmPage(h.layout(), selfstate.VerifyEmailChangeConfirmState{
		Token:    token,
		NewEmail: string(peeked.Payload),
	}))
}

func (h *Handler) doVerifyEmailChange(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Referrer-Policy", "no-referrer")
	if err := httpx.ParseForm(w, r, maxVerifyFormBytes); err != nil {
		httpx.RenderHTML(w, r, h.logger, h.theme.AccountEmailChangeResultPage(h.layout(), selfstate.EmailChangeResultState{Kind: selfstate.EmailChangeResultInvalid}))
		return
	}

	token := r.PostFormValue(fieldToken)
	if token == "" {
		httpx.RenderHTML(w, r, h.logger, h.theme.AccountEmailChangeResultPage(h.layout(), selfstate.EmailChangeResultState{Kind: selfstate.EmailChangeResultInvalid}))
		return
	}

	user, err := h.svc.ConsumeEmailChange(r.Context(), token)
	if err != nil {
		state := selfstate.EmailChangeResultState{Kind: selfstate.EmailChangeResultInvalid}
		switch {
		case errors.Is(err, actiontokendomain.ErrTokenExpired):
			state.Kind = selfstate.EmailChangeResultExpired
		case errors.Is(err, actiontokendomain.ErrTokenAlreadyUsed):
			state.Kind = selfstate.EmailChangeResultAlready
		case errors.Is(err, actiontokendomain.ErrTokenInvalid):
			state.Kind = selfstate.EmailChangeResultInvalid
		case errors.Is(err, domain.ErrEmailTaken):
			state.Kind = selfstate.EmailChangeResultTaken
		case errors.Is(err, domain.ErrAccountPermaBanned),
			errors.Is(err, domain.ErrAccountTempBanned),
			errors.Is(err, domain.ErrAccountDeleted):
			state.Kind = selfstate.EmailChangeResultAccountRestricted
		default:
			h.logger.Error("verify email change", "err", err)
			state.Kind = selfstate.EmailChangeResultInvalid
		}
		httpx.RenderHTML(w, r, h.logger, h.theme.AccountEmailChangeResultPage(h.layout(), state))
		return
	}

	httpx.RenderHTML(w, r, h.logger, h.theme.AccountEmailChangeResultPage(h.layout(), selfstate.EmailChangeResultState{
		Kind:     selfstate.EmailChangeResultSuccess,
		NewEmail: user.Email,
	}))
}
