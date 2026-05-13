package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/account/domain"
	"github.com/hayakawakaki/go-racp/internal/httpx"
)

func RequireLogin(sessSvc SessionValidator, logger *slog.Logger, secure bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(SessionCookieName)
			if err != nil || cookie.Value == "" {
				httpx.Redirect(w, r, "/login")
				return
			}

			sess, err := sessSvc.Validate(r.Context(), cookie.Value)
			if err != nil {
				if errors.Is(err, domain.ErrSessionNotFound) || errors.Is(err, domain.ErrSessionExpired) {
					ClearSessionCookie(w, secure)
					httpx.Redirect(w, r, "/login")
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
