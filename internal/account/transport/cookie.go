package transport

import (
	"net/http"
	"time"

	"github.com/hayakawakaki/go-racp/internal/account/transport/middleware"
)

// setSessionCookie sets the session cookie used for authentication with the provided token and TTL. The cookie is HttpOnly, SameSite=Lax, uses Path "/", and its Secure flag is controlled externally.
func setSessionCookie(w http.ResponseWriter, token string, ttl time.Duration, secure bool) {
	//nolint:gosec // G124: Secure is env-driven (true in production, false in development for HTTP localhost). Wired in plugin.go.
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(ttl.Seconds()),
	})
}
