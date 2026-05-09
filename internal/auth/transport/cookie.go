package transport

import (
	"net/http"
	"time"
)

const sessionCookieName = "racp_session"

// setSessionCookie sets the session cookie used for authentication with the provided token and TTL; the cookie is HttpOnly, SameSite=Lax, uses Path "/", and its Secure flag is controlled externally.
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

// clearSessionCookie clears the HTTP session cookie (`racp_session`) so the browser removes it.
// The secure parameter controls whether the cookie's Secure attribute is set.
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
