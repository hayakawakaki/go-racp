package transport

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/account/domain"
	"github.com/hayakawakaki/go-racp/internal/httpx"
)

type ctxKey int

const sessionKey ctxKey = 0

type SessionValidator interface {
	Validate(ctx context.Context, rawToken string) (*domain.Session, error)
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

// RequireAuth returns an HTTP middleware that enforces a valid session.
// Requests with a missing or invalid session are redirected to /login (HTMX requests receive an HX-Redirect header with 204 No Content).
// The secure flag controls the attributes used when clearing the session cookie.
func RequireAuth(sessSvc SessionValidator, logger *slog.Logger, secure bool) func(http.HandlerFunc) http.HandlerFunc {
	return sessionMiddleware(sessSvc, logger, secure, func(w http.ResponseWriter, r *http.Request, _ http.HandlerFunc) {
		unauthorized(w, r)
	})
}

// WithSession returns an HTTP middleware that injects a validated *domain.Session into the request context when a valid session cookie is present and allows the request to proceed unchanged when no valid session exists.
func WithSession(sessSvc SessionValidator, logger *slog.Logger, secure bool) func(http.HandlerFunc) http.HandlerFunc {
	return sessionMiddleware(sessSvc, logger, secure, func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		next(w, r)
	})
}

// sessionMiddleware creates an HTTP middleware that validates a session cookie and, when valid,
// stores the resulting *domain.Session in the request context before calling the next handler.
// If the cookie is missing or validation fails it calls onNoSession; on validation failure it
// also clears the session cookie and logs the error unless it is domain.ErrSessionNotFound or
// domain.ErrSessionExpired.
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

// unauthorized redirects the client to the login page.
// For HTMX requests it sets the "HX-Redirect" response header to "/login" and returns HTTP 204 No Content.
// For non-HTMX requests it issues an HTTP 303 See Other redirect to "/login".
func unauthorized(w http.ResponseWriter, r *http.Request) {
	httpx.Redirect(w, r, "/login")
}
