package transport

import (
	"errors"
	"net/http"

	"github.com/a-h/templ"
	"github.com/hayakawakaki/go-racp/internal/platform/actiontoken/domain"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
)

func (h *Handler) validateTokenLink(w http.ResponseWriter, r *http.Request, kind domain.Action, op string, expired templ.Component) (string, bool) {
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")

	token := r.URL.Query().Get("token")
	if token == "" {
		httpx.Render404(w, r, h.logger, h.layout())
		return "", false
	}

	if _, err := h.svc.Peek(r.Context(), kind, token); err != nil {
		if errors.Is(err, domain.ErrTokenExpired) {
			httpx.RenderHTML(w, r, h.logger, expired)
			return "", false
		}
		if !errors.Is(err, domain.ErrTokenInvalid) && !errors.Is(err, domain.ErrTokenAlreadyUsed) {
			h.logger.Error(op, "err", err)
		}
		httpx.Render404(w, r, h.logger, h.layout())
		return "", false
	}

	return token, true
}
