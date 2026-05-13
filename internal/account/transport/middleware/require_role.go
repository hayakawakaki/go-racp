package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/account/domain"
	"github.com/hayakawakaki/go-racp/internal/httpx"
)

func roleAllowed(role domain.Role, anyAllowed bool, allowSet map[domain.Role]struct{}) bool {
	if role == domain.RoleAdmin || anyAllowed {
		return true
	}
	_, ok := allowSet[role]
	return ok
}

func rejectUnauthenticated(w http.ResponseWriter, r *http.Request, logger *slog.Logger, hidden bool, layout httpx.Layout) {
	if hidden {
		httpx.Render404(w, r, logger, layout)
		return
	}
	httpx.Redirect(w, r, "/login")
}

func rejectForbidden(w http.ResponseWriter, r *http.Request, logger *slog.Logger, hidden bool, layout httpx.Layout) {
	if hidden {
		httpx.Render404(w, r, logger, layout)
		return
	}
	http.Error(w, "forbidden", http.StatusForbidden)
}

func handleSessionError(w http.ResponseWriter, r *http.Request, err error, logger *slog.Logger, secure, hidden bool, layout httpx.Layout) {
	if errors.Is(err, domain.ErrSessionNotFound) || errors.Is(err, domain.ErrSessionExpired) {
		ClearSessionCookie(w, secure)
		rejectUnauthenticated(w, r, logger, hidden, layout)
		return
	}
	logger.Error("require_role: session validate", "err", err)
	http.Error(w, "internal server error", http.StatusInternalServerError)
}

func requireRoleCore(
	sessSvc SessionValidator,
	users UserLookup,
	resolver domain.RoleResolver,
	logger *slog.Logger,
	secure, hidden bool,
	layout httpx.Layout,
	allowed []domain.Role,
) func(http.Handler) http.Handler {
	allowSet := make(map[domain.Role]struct{}, len(allowed))
	for _, role := range allowed {
		allowSet[role] = struct{}{}
	}
	_, anyAllowed := allowSet[domain.RoleAny]

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(SessionCookieName)
			if err != nil || cookie.Value == "" {
				rejectUnauthenticated(w, r, logger, hidden, layout)
				return
			}

			sess, err := sessSvc.Validate(r.Context(), cookie.Value)
			if err != nil {
				handleSessionError(w, r, err, logger, secure, hidden, layout)
				return
			}

			user, err := users.GetByID(r.Context(), sess.UserID)
			if err != nil {
				logger.Error("require_role: load user", "err", err, "userID", sess.UserID)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}

			role := resolver.Resolve(user.GroupID)
			if !roleAllowed(role, anyAllowed, allowSet) {
				rejectForbidden(w, r, logger, hidden, layout)
				return
			}

			ctx := context.WithValue(r.Context(), sessionKey, sess)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireRole(
	sessSvc SessionValidator,
	users UserLookup,
	resolver domain.RoleResolver,
	logger *slog.Logger,
	secure bool,
	allowed ...domain.Role,
) func(http.Handler) http.Handler {
	return requireRoleCore(sessSvc, users, resolver, logger, secure, false, httpx.Layout{}, allowed)
}

func RequireRoleHidden(
	sessSvc SessionValidator,
	users UserLookup,
	resolver domain.RoleResolver,
	logger *slog.Logger,
	secure bool,
	layout httpx.Layout,
	allowed ...domain.Role,
) func(http.Handler) http.Handler {
	return requireRoleCore(sessSvc, users, resolver, logger, secure, true, layout, allowed)
}
