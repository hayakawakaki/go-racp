package transport

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/auth/domain"
	"github.com/hayakawakaki/go-racp/internal/httpx"
)

type ctxKey int

const sessionKey ctxKey = 0

type SessionValidator interface {
	Validate(ctx context.Context, rawToken string) (*domain.Session, error)
}

func SessionFromContext(ctx context.Context) (*domain.Session, bool) {
	s, ok := ctx.Value(sessionKey).(*domain.Session)
	return s, ok
}

func ContextWithSession(ctx context.Context, sess *domain.Session) context.Context {
	return context.WithValue(ctx, sessionKey, sess)
}

func RequireAuth(sessSvc SessionValidator, logger *slog.Logger, secure bool) func(http.HandlerFunc) http.HandlerFunc {
	return sessionMiddleware(sessSvc, logger, secure, func(w http.ResponseWriter, r *http.Request, _ http.HandlerFunc) {
		unauthorized(w, r)
	})
}

func WithSession(sessSvc SessionValidator, logger *slog.Logger, secure bool) func(http.HandlerFunc) http.HandlerFunc {
	return sessionMiddleware(sessSvc, logger, secure, func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		next(w, r)
	})
}

func sessionMiddleware(
	sessSvc SessionValidator,
	logger *slog.Logger,
	secure bool,
	onNoSession func(http.ResponseWriter, *http.Request, http.HandlerFunc),
) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			c, err := r.Cookie(sessionCookieName)
			if err != nil {
				onNoSession(w, r, next)
				return
			}
			sess, err := sessSvc.Validate(r.Context(), c.Value)
			if err != nil {
				clearSessionCookie(w, secure)
				if !errors.Is(err, domain.ErrSessionNotFound) && !errors.Is(err, domain.ErrSessionExpired) {
					logger.Error("session validate", "err", err)
				}
				onNoSession(w, r, next)
				return
			}
			ctx := context.WithValue(r.Context(), sessionKey, sess)
			next(w, r.WithContext(ctx))
		}
	}
}

func unauthorized(w http.ResponseWriter, r *http.Request) {
	if httpx.IsHTMX(r) {
		w.Header().Set("HX-Redirect", "/login")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
