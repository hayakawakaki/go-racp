package transport

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/account/domain"
)

func RequireLogin(sessSvc sessionService, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(sessionCookieName)
			if err != nil || cookie.Value == "" {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			sess, err := sessSvc.Validate(r.Context(), cookie.Value)
			if err != nil {
				if errors.Is(err, domain.ErrSessionNotFound) || errors.Is(err, domain.ErrSessionExpired) {
					http.Redirect(w, r, "/login", http.StatusSeeOther)
					return
				}
				logger.Error("require_login: validate", "err", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			ctx := context.WithValue(r.Context(), sessionKey, sess)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
