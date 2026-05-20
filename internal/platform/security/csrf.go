package security

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"mime"
	"net/http"
)

const (
	csrfCookieName   = "racp_csrf"
	csrfHeaderName   = "X-CSRF-Token"
	csrfFormField    = "_csrf"
	csrfCookieMaxAge = 60 * 60 * 12
	csrfMinSecretLen = 32
)

type SessionFingerprint func(ctx context.Context) ([]byte, bool)

type CSRFOptions struct {
	GetSessionFinger SessionFingerprint
	Secret           []byte
	Secure           bool
}

func CSRF(opts CSRFOptions) func(http.Handler) http.Handler {
	if len(opts.Secret) < csrfMinSecretLen {
		panic("security: CSRF secret must be >= 32 bytes")
	}
	if opts.GetSessionFinger == nil {
		panic("security: CSRF GetSessionFinger is required")
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			expected, err := resolveExpectedToken(r, opts)
			if err != nil {
				http.Error(w, "csrf init failed", http.StatusInternalServerError)
				return
			}
			ensureCSRFCookie(w, r, expected, opts.Secure)
			if !isSafeMethod(r.Method) {
				if !tokenValid(r, expected) {
					http.Error(w, "csrf token invalid", http.StatusForbidden)
					return
				}
			}
			ctx := contextWithToken(r.Context(), expected)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func resolveExpectedToken(r *http.Request, opts CSRFOptions) (string, error) {
	if finger, ok := opts.GetSessionFinger(r.Context()); ok {
		mac := hmac.New(sha256.New, opts.Secret)
		mac.Write(finger)
		return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
	}
	if c, err := r.Cookie(csrfCookieName); err == nil && c.Value != "" {
		return c.Value, nil
	}
	buf := make([]byte, csrfMinSecretLen)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("security.resolveExpectedToken: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func ensureCSRFCookie(w http.ResponseWriter, r *http.Request, token string, secure bool) {
	if c, err := r.Cookie(csrfCookieName); err == nil && c.Value == token {
		return
	}
	//nolint:gosec // G124: HttpOnly off so JS can read for HTMX hx-headers
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   csrfCookieMaxAge,
	})
}

func tokenValid(r *http.Request, expected string) bool {
	if h := r.Header.Get(csrfHeaderName); h != "" {
		return constantTimeEqual(h, expected)
	}
	ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if ct != "application/x-www-form-urlencoded" && ct != "multipart/form-data" {
		return false
	}
	if err := r.ParseForm(); err != nil {
		return false
	}
	return constantTimeEqual(r.PostForm.Get(csrfFormField), expected)
}

func constantTimeEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
