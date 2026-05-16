package transport

import (
	"context"
	"errors"
	"net/http"

	"github.com/a-h/templ"
	actiontokendomain "github.com/hayakawakaki/go-racp/internal/actiontoken/domain"
	"github.com/hayakawakaki/go-racp/internal/httpx"
)

type tokenPeek func(ctx context.Context, rawToken string) (*actiontokendomain.ActionToken, error)

func (h *Handler) validateTokenLink(w http.ResponseWriter, r *http.Request, peek tokenPeek, op string, expired templ.Component) (string, bool) {
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")

	token := r.URL.Query().Get("token")
	if token == "" {
		httpx.Render404(w, r, h.logger, h.layout())
		return "", false
	}

	if _, err := peek(r.Context(), token); err != nil {
		if errors.Is(err, actiontokendomain.ErrTokenExpired) {
			httpx.RenderHTML(w, r, h.logger, expired)
			return "", false
		}
		if !errors.Is(err, actiontokendomain.ErrTokenInvalid) && !errors.Is(err, actiontokendomain.ErrTokenAlreadyUsed) {
			h.logger.Error(op, "err", err)
		}
		httpx.Render404(w, r, h.logger, h.layout())
		return "", false
	}

	return token, true
}
