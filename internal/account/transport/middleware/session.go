package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/account/domain"
)

const SessionCookieName = "racp_session"

type ctxKey int

const sessionKey ctxKey = 0

type SessionValidator interface {
	Validate(ctx context.Context, rawToken string) (*domain.Session, error)
	Destroy(ctx context.Context, rawToken string) error
}

// SessionFromContext retrieves the *domain.Session value stored in ctx under the package's session key and reports whether it was present and of the expected type.
func SessionFromContext(ctx context.Context) (*domain.Session, bool) {
	s, ok := ctx.Value(sessionKey).(*domain.Session)
	return s, ok
}

// ContextWithSession returns a new context that stores the provided *domain.Session
// under the package session key so handlers and middleware can access the session.
func ContextWithSession(ctx context.Context, sess *domain.Session) context.Context {
	return context.WithValue(ctx, sessionKey, sess)
}

// ClearSessionCookie clears the HTTP session cookie (`racp_session`) so the browser removes it.
// The secure parameter controls whether the cookie's Secure attribute is set.
func ClearSessionCookie(w http.ResponseWriter, secure bool) {
	//nolint:gosec // G124: Secure is env-driven (true in production, false in development for HTTP localhost). Wired in plugin.go.
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// WithSession returns an HTTP middleware that injects a validated *domain.Session into the request context when a valid session cookie is present and allows the request to proceed unchanged when no valid session exists.
func WithSession(sessSvc SessionValidator, logger *slog.Logger, secure bool) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			c, err := r.Cookie(SessionCookieName)
			if err != nil {
				next(w, r)
				return
			}

			sess, err := sessSvc.Validate(r.Context(), c.Value)
			if err != nil {
				ClearSessionCookie(w, secure)
				if !errors.Is(err, domain.ErrSessionNotFound) && !errors.Is(err, domain.ErrSessionExpired) {
					logger.Error("session validate", "err", err)
				}
				next(w, r)
				return
			}

			ctx := context.WithValue(r.Context(), sessionKey, sess)
			next(w, r.WithContext(ctx))
		}
	}
}
