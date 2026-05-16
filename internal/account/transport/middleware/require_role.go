package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/hayakawakaki/go-racp/internal/account/app"
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

//nolint:cyclop // sequential gating, splitting would obscure the flow
func requireRoleCore(
	sessSvc SessionValidator,
	users UserLookup,
	resolver domain.RoleResolver,
	logger *slog.Logger,
	secure, hidden bool,
	layout httpx.Layout,
	fullAccess bool,
	allowed []domain.Role,
) func(http.Handler) http.Handler {
	allowSet := make(map[domain.Role]struct{}, len(allowed))
	for _, role := range allowed {
		allowSet[role] = struct{}{}
	}
	_, anyAllowed := allowSet[domain.RoleAuthenticated]

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
			if errors.Is(err, domain.ErrUserNotFound) {
				snap := &AccountSnapshot{UserID: sess.UserID, Tier: app.TierDeleted}
				rejectBanned(w, r, logger, secure, hidden, layout, "deleted", snap, cookie.Value, sessSvc)
				return
			}
			if err != nil {
				logger.Error("require_role: load user", "err", err, "userID", sess.UserID)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}

			tier := app.ClassifyTier(user.State, user.UnbanTime, time.Now())
			snap := &AccountSnapshot{UserID: user.ID, Tier: tier, UnbanTime: user.UnbanTime}

			switch tier {
			case app.TierPermaBanned:
				rejectBanned(w, r, logger, secure, hidden, layout, "banned", snap, cookie.Value, sessSvc)
				return
			case app.TierUnverified:
				rejectUnverified(w, r, logger, hidden, layout)
				return
			case app.TierTempBanned:
				if fullAccess {
					rejectTempBanned(w, r, logger, hidden, layout)
					return
				}
			case app.TierActive, app.TierDeleted:
			}

			role := resolver.Resolve(user.GroupID)
			if !roleAllowed(role, anyAllowed, allowSet) {
				rejectForbidden(w, r, logger, hidden, layout)
				return
			}

			ctx := context.WithValue(r.Context(), sessionKey, sess)
			ctx = ContextWithSnapshot(ctx, snap)
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
	fullAccess bool,
	allowed ...domain.Role,
) func(http.Handler) http.Handler {
	return requireRoleCore(sessSvc, users, resolver, logger, secure, false, httpx.Layout{}, fullAccess, allowed)
}

// RequireRoleHidden behaves like RequireRole but renders a 404 in place of 401/403 so the route's existence is not disclosed to unauthorized callers.
func RequireRoleHidden(
	sessSvc SessionValidator,
	users UserLookup,
	resolver domain.RoleResolver,
	logger *slog.Logger,
	secure bool,
	layout httpx.Layout,
	fullAccess bool,
	allowed ...domain.Role,
) func(http.Handler) http.Handler {
	return requireRoleCore(sessSvc, users, resolver, logger, secure, true, layout, fullAccess, allowed)
}

func rejectBanned(w http.ResponseWriter, r *http.Request, logger *slog.Logger, secure, hidden bool, layout httpx.Layout, notice string, snap *AccountSnapshot, sessRaw string, sessSvc SessionValidator) {
	if err := sessSvc.Destroy(r.Context(), sessRaw); err != nil {
		logger.Error("require_role: session destroy after ban kick", "err", err)
	}
	ClearSessionCookie(w, secure)
	logger.Info("session terminated by ban gate",
		"account_id", snap.UserID,
		"tier", snap.Tier.String(),
		"unban_time", snap.UnbanTime,
	)
	if hidden {
		httpx.Render404(w, r, logger, layout)
		return
	}
	httpx.Redirect(w, r, "/login?notice="+notice)
}

func rejectTempBanned(w http.ResponseWriter, r *http.Request, logger *slog.Logger, hidden bool, layout httpx.Layout) {
	if hidden {
		httpx.Render404(w, r, logger, layout)
		return
	}
	httpx.Redirect(w, r, "/account?notice=ban_blocked")
}

func rejectUnverified(w http.ResponseWriter, r *http.Request, logger *slog.Logger, hidden bool, layout httpx.Layout) {
	if hidden {
		httpx.Render404(w, r, logger, layout)
		return
	}
	httpx.Redirect(w, r, "/verify-account")
}
