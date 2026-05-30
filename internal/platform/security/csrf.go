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
	"strings"
)

const (
	csrfCookieBase   = "racp_csrf"
	csrfCookieHost   = "__Host-racp_csrf"
	csrfHeaderName   = "X-CSRF-Token"
	csrfFormField    = "_csrf"
	csrfCookieMaxAge = 60 * 60 * 12
	csrfMinSecretLen = 32
	csrfRandomLen    = 32
	csrfMaxFormBytes = 128 << 10
)

type SessionFingerprint func(ctx context.Context) ([]byte, bool)

type CSRFOptions struct {
	GetSessionFinger SessionFingerprint
	OpenRoutes       *RouteMatcher
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
			if opts.OpenRoutes.Allows(r) {
				next.ServeHTTP(w, r)
				return
			}

			expected, err := resolveExpectedToken(r, opts)
			if err != nil {
				http.Error(w, "csrf init failed", http.StatusInternalServerError)
				return
			}

			if expected != "" {
				ensureCSRFCookie(w, r, expected, opts.Secure)
			}

			if !isSafeMethod(r.Method) {
				if expected == "" || !tokenValid(w, r, expected) {
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
	if fingerprint, ok := opts.GetSessionFinger(r.Context()); ok {
		mac := hmac.New(sha256.New, opts.Secret)
		mac.Write(fingerprint)

		return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
	}

	name := cookieName(opts.Secure)

	if cookie, err := r.Cookie(name); err == nil && cookie.Value != "" {
		if verifyAnonymousToken(cookie.Value, opts.Secret) {
			return cookie.Value, nil
		}
	}

	if !shouldMintAnonymousToken(r) {
		return "", nil
	}

	return newAnonymousToken(opts.Secret)
}

func newAnonymousToken(secret []byte) (string, error) {
	buffer := make([]byte, csrfRandomLen)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("security.newAnonymousToken: %w", err)
	}

	raw := base64.RawURLEncoding.EncodeToString(buffer)

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(raw))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return raw + "." + signature, nil
}

func verifyAnonymousToken(value string, secret []byte) bool {
	raw, signature, ok := strings.Cut(value, ".")
	if !ok || raw == "" || signature == "" {
		return false
	}

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(raw))
	want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return subtle.ConstantTimeCompare([]byte(signature), []byte(want)) == 1
}

func shouldMintAnonymousToken(r *http.Request) bool {
	switch r.Header.Get("Sec-Fetch-Site") {
	case "cross-site", "same-site":
		return isTopLevelNavigation(r)
	}

	return true
}

func isTopLevelNavigation(r *http.Request) bool {
	return r.Header.Get("Sec-Fetch-Mode") == "navigate" &&
		r.Header.Get("Sec-Fetch-Dest") == "document"
}

func cookieName(secure bool) string {
	if secure {
		return csrfCookieHost
	}

	return csrfCookieBase
}

func ensureCSRFCookie(w http.ResponseWriter, r *http.Request, token string, secure bool) {
	name := cookieName(secure)

	if cookie, err := r.Cookie(name); err == nil && cookie.Value == token {
		return
	}

	//nolint:gosec // G124: Secure tied to env via opts
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   csrfCookieMaxAge,
	})
}

func tokenValid(w http.ResponseWriter, r *http.Request, expected string) bool {
	if header := r.Header.Get(csrfHeaderName); header != "" {
		return constantTimeEqual(header, expected)
	}

	contentType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if contentType != "application/x-www-form-urlencoded" && contentType != "multipart/form-data" {
		return false
	}

	r.Body = http.MaxBytesReader(w, r.Body, csrfMaxFormBytes)
	if err := r.ParseForm(); err != nil {
		return false
	}

	return constantTimeEqual(r.PostForm.Get(csrfFormField), expected)
}

func constantTimeEqual(left, right string) bool {
	if len(left) != len(right) {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}
