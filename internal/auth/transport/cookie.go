package transport

import (
	"net/http"
	"time"
)

const sessionCookieName = "racp_session"

func setSessionCookie(w http.ResponseWriter, token string, ttl time.Duration, secure bool) {
	//nolint:gosec // G124: Secure is env-driven (true in production, false in development for HTTP localhost). Wired in plugin.go.
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(ttl.Seconds()),
	})
}

func clearSessionCookie(w http.ResponseWriter, secure bool) {
	//nolint:gosec // G124: Secure is env-driven (true in production, false in development for HTTP localhost). Wired in plugin.go.
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
