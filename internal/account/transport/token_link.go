package transport

import (
	"context"
	"errors"
	"net/http"

	"github.com/a-h/templ"
	"github.com/hayakawakaki/go-racp/internal/actiontoken"
	"github.com/hayakawakaki/go-racp/internal/httpx"
)

type tokenPeek func(ctx context.Context, rawToken string) (*actiontoken.ActionToken, error)

func (h *Handler) validateTokenLink(w http.ResponseWriter, r *http.Request, peek tokenPeek, op string, expired templ.Component) (string, bool) {
	w.Header().Set("Referrer-Policy", "no-referrer")
	token := r.URL.Query().Get("token")
	if token == "" {
		http.NotFound(w, r)
		return "", false
	}
	if _, err := peek(r.Context(), token); err != nil {
		if errors.Is(err, actiontoken.ErrTokenExpired) {
			httpx.RenderHTML(w, r, h.logger, expired)
			return "", false
		}
		if !errors.Is(err, actiontoken.ErrTokenInvalid) && !errors.Is(err, actiontoken.ErrTokenAlreadyUsed) {
			h.logger.Error(op, "err", err)
		}
		http.NotFound(w, r)
		return "", false
	}
	return token, true
}
