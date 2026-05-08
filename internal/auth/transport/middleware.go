package transport

import (
	"context"
	"errors"
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/auth/domain"
	"github.com/hayakawakaki/go-racp/internal/httpx"
)

type ctxKey int

const sessionKey ctxKey = 0

func SessionFromContext(ctx context.Context) (*domain.Session, bool) {
	s, ok := ctx.Value(sessionKey).(*domain.Session)
	return s, ok
}

func ContextWithSession(ctx context.Context, sess *domain.Session) context.Context {
	return context.WithValue(ctx, sessionKey, sess)
}

func (h *Handler) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookieName)
		if err != nil {
			h.unauthorized(w, r)
			return
		}
		sess, err := h.sessSvc.Validate(r.Context(), c.Value)
		if err != nil {
			clearSessionCookie(w, h.secure)
			if !errors.Is(err, domain.ErrSessionNotFound) && !errors.Is(err, domain.ErrSessionExpired) {
				h.logger.Error("session validate", "err", err)
			}
			h.unauthorized(w, r)
			return
		}
		ctx := context.WithValue(r.Context(), sessionKey, sess)
		next(w, r.WithContext(ctx))
	}
}

func (h *Handler) WithSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookieName)
		if err != nil {
			next(w, r)
			return
		}
		sess, err := h.sessSvc.Validate(r.Context(), c.Value)
		if err != nil {
			clearSessionCookie(w, h.secure)
			if !errors.Is(err, domain.ErrSessionNotFound) && !errors.Is(err, domain.ErrSessionExpired) {
				h.logger.Error("session validate", "err", err)
			}
			next(w, r)
			return
		}
		ctx := context.WithValue(r.Context(), sessionKey, sess)
		next(w, r.WithContext(ctx))
	}
}

func (h *Handler) unauthorized(w http.ResponseWriter, r *http.Request) {
	if httpx.IsHTMX(r) {
		w.Header().Set("HX-Redirect", "/login")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
