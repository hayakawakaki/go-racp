package transport

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/hayakawakaki/go-racp/internal/auth/domain"
)

const verifyAccountPath = "/verify-account"

func RequireVerified(
	sessSvc sessionService,
	users userLookup,
	logger *slog.Logger,
	allowPrefixes []string,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isAllowed(r.URL.Path, allowPrefixes) {
				next.ServeHTTP(w, r)
				return
			}
			cookie, err := r.Cookie(sessionCookieName)
			if err != nil || cookie.Value == "" {
				next.ServeHTTP(w, r)
				return
			}
			sess, err := sessSvc.Validate(r.Context(), cookie.Value)
			if err != nil {
				if errors.Is(err, domain.ErrSessionNotFound) || errors.Is(err, domain.ErrSessionExpired) {
					next.ServeHTTP(w, r)
					return
				}
				logger.Error("require_verified: session validate", "err", err)
				next.ServeHTTP(w, r)
				return
			}
			user, err := users.GetByID(r.Context(), sess.UserID)
			if err != nil {
				logger.Error("require_verified: load user", "err", err)
				next.ServeHTTP(w, r)
				return
			}
			if user.GroupID == 5 {
				http.Redirect(w, r, verifyAccountPath, http.StatusSeeOther)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func isAllowed(path string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}
