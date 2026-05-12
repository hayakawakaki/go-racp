package transport

import (
	"errors"
	"net/http"

	authdomain "github.com/hayakawakaki/go-racp/internal/account/domain"
	"github.com/hayakawakaki/go-racp/internal/actiontoken"
	"github.com/hayakawakaki/go-racp/internal/httpx"
)

func (h *Handler) doVerifyEmailChange(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Referrer-Policy", "no-referrer")
	token := r.URL.Query().Get("token")
	if token == "" {
		httpx.RenderHTML(w, r, h.logger, emailChangeResultPage(h.layout(), EmailChangeResultState{Kind: EmailChangeResultInvalid}))
		return
	}
	user, err := h.svc.ConsumeEmailChange(r.Context(), token)
	if err != nil {
		state := EmailChangeResultState{Kind: EmailChangeResultInvalid}
		switch {
		case errors.Is(err, actiontoken.ErrTokenExpired):
			state.Kind = EmailChangeResultExpired
		case errors.Is(err, actiontoken.ErrTokenAlreadyUsed):
			state.Kind = EmailChangeResultAlready
		case errors.Is(err, actiontoken.ErrTokenInvalid):
			state.Kind = EmailChangeResultInvalid
		case errors.Is(err, authdomain.ErrEmailTaken):
			state.Kind = EmailChangeResultTaken
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
