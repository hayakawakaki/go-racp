package security

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

var testCSRFSecret = []byte("test-secret-32-bytes-fixed-key__")

func TestCSRF_PanicsOnShortSecret(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic for short secret")
		}
	}()

	CSRF(CSRFOptions{
		Secret:           []byte("too-short"),
		GetSessionFinger: noSession,
	})
}

func TestCSRF_PanicsOnMissingSessionFinger(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic for missing GetSessionFinger")
		}
	}()

	CSRF(CSRFOptions{Secret: testCSRFSecret})
}

func TestCSRF_VerifyAnonymousToken(t *testing.T) {
	t.Parallel()

	validToken := mustNewAnonToken(t, testCSRFSecret)
	raw, sig, _ := strings.Cut(validToken, ".")

	tests := []struct {
		name  string
		token string
		want  bool
	}{
		{name: "valid signed token", token: validToken, want: true},
		{name: "missing separator", token: "rawvaluewithoutdot", want: false},
		{name: "empty raw", token: "." + sig, want: false},
		{name: "empty signature", token: raw + ".", want: false},
		{name: "tampered raw", token: "tampered." + sig, want: false},
		{name: "tampered signature", token: raw + ".tampered-signature-bytes", want: false},
		{name: "completely empty", token: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := verifyAnonymousToken(tt.token, testCSRFSecret); got != tt.want {
				t.Errorf("verifyAnonymousToken(%q) = %v, want %v", tt.token, got, tt.want)
			}
		})
	}
}

func TestCSRF_NewAnonymousTokenRoundTrip(t *testing.T) {
	t.Parallel()

	tok, err := newAnonymousToken(testCSRFSecret)
	if err != nil {
		t.Fatalf("newAnonymousToken: %v", err)
	}
	if !strings.Contains(tok, ".") {
		t.Errorf("token missing '.' separator: %q", tok)
	}
	if !verifyAnonymousToken(tok, testCSRFSecret) {
		t.Errorf("freshly minted token failed verification: %q", tok)
	}
	other := []byte("different-secret-also-32-bytes!!")
	if verifyAnonymousToken(tok, other) {
		t.Errorf("token verified under wrong secret")
	}
}

func TestCSRF_CookieNameByEnvironment(t *testing.T) {
	t.Parallel()

	if got := cookieName(true); got != "__Host-racp_csrf" {
		t.Errorf("cookieName(true) = %q, want __Host-racp_csrf", got)
	}
	if got := cookieName(false); got != "racp_csrf" {
		t.Errorf("cookieName(false) = %q, want racp_csrf", got)
	}
}

func TestCSRF_ShouldMintAnonymousToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		secFetch string
		wantMint bool
	}{
		{name: "absent header allows mint", secFetch: "", wantMint: true},
		{name: "none allows mint", secFetch: "none", wantMint: true},
		{name: "same-origin allows mint", secFetch: "same-origin", wantMint: true},
		{name: "same-site blocks mint", secFetch: "same-site", wantMint: false},
		{name: "cross-site blocks mint", secFetch: "cross-site", wantMint: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			if tt.secFetch != "" {
				req.Header.Set("Sec-Fetch-Site", tt.secFetch)
			}
			if got := shouldMintAnonymousToken(req); got != tt.wantMint {
				t.Errorf("shouldMintAnonymousToken = %v, want %v", got, tt.wantMint)
			}
		})
	}
}

func TestCSRF_SafeMethodsBypassValidation(t *testing.T) {
	t.Parallel()

	methods := []string{http.MethodGet, http.MethodHead, http.MethodOptions}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			t.Parallel()

			called := false
			handler := CSRF(CSRFOptions{
				Secret:           testCSRFSecret,
				GetSessionFinger: noSession,
			})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(method, "/", http.NoBody)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if !called {
				t.Errorf("next handler not invoked for safe method %s", method)
			}
			if rr.Code != http.StatusOK {
				t.Errorf("status = %d, want 200", rr.Code)
			}
		})
	}
}

func TestCSRF_StateChangingFlows(t *testing.T) {
	t.Parallel()

	validAnonToken := mustNewAnonToken(t, testCSRFSecret)
	otherSecretToken := mustNewAnonToken(t, []byte("different-secret-also-32-bytes!!"))
	fingerprint := []byte("session-token-hash-32-byte-fixed")
	hmacToken := sessionExpected(testCSRFSecret, fingerprint)

	tests := []struct {
		name        string
		method      string
		cookieValue string
		headerValue string
		formValue   string
		secFetch    string
		finger      []byte
		wantStatus  int
	}{
		{
			name:        "anonymous valid form field passes",
			method:      http.MethodPost,
			cookieValue: validAnonToken,
			formValue:   validAnonToken,
			secFetch:    "same-origin",
			wantStatus:  http.StatusOK,
		},
		{
			name:        "anonymous valid header passes",
			method:      http.MethodPost,
			cookieValue: validAnonToken,
			headerValue: validAnonToken,
			secFetch:    "same-origin",
			wantStatus:  http.StatusOK,
		},
		{
			name:        "anonymous no token rejected",
			method:      http.MethodPost,
			cookieValue: validAnonToken,
			secFetch:    "same-origin",
			wantStatus:  http.StatusForbidden,
		},
		{
			name:        "anonymous wrong form field rejected",
			method:      http.MethodPost,
			cookieValue: validAnonToken,
			formValue:   "wrong-value",
			secFetch:    "same-origin",
			wantStatus:  http.StatusForbidden,
		},
		{
			name:        "anonymous tampered cookie ignored fresh minted then 403 wrong field",
			method:      http.MethodPost,
			cookieValue: "tampered.value",
			formValue:   "tampered.value",
			secFetch:    "same-origin",
			wantStatus:  http.StatusForbidden,
		},
		{
			name:        "anonymous cookie from other secret rejected",
			method:      http.MethodPost,
			cookieValue: otherSecretToken,
			formValue:   otherSecretToken,
			secFetch:    "same-origin",
			wantStatus:  http.StatusForbidden,
		},
		{
			name:       "cross-site no cookie rejected without minting",
			method:     http.MethodPost,
			secFetch:   "cross-site",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "session HMAC form field passes",
			method:     http.MethodPost,
			formValue:  hmacToken,
			secFetch:   "same-origin",
			finger:     fingerprint,
			wantStatus: http.StatusOK,
		},
		{
			name:        "session HMAC header passes",
			method:      http.MethodPost,
			headerValue: hmacToken,
			secFetch:    "same-origin",
			finger:      fingerprint,
			wantStatus:  http.StatusOK,
		},
		{
			name:       "session wrong token rejected",
			method:     http.MethodPost,
			formValue:  "wrong",
			secFetch:   "same-origin",
			finger:     fingerprint,
			wantStatus: http.StatusForbidden,
		},
		{
			name:        "PUT with valid token passes",
			method:      http.MethodPut,
			cookieValue: validAnonToken,
			headerValue: validAnonToken,
			secFetch:    "same-origin",
			wantStatus:  http.StatusOK,
		},
		{
			name:        "DELETE with valid token passes",
			method:      http.MethodDelete,
			cookieValue: validAnonToken,
			headerValue: validAnonToken,
			secFetch:    "same-origin",
			wantStatus:  http.StatusOK,
		},
		{
			name:        "PATCH with valid token passes",
			method:      http.MethodPatch,
			cookieValue: validAnonToken,
			headerValue: validAnonToken,
			secFetch:    "same-origin",
			wantStatus:  http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			finger := tt.finger
			opts := CSRFOptions{
				Secret: testCSRFSecret,
				GetSessionFinger: func(_ context.Context) ([]byte, bool) {
					if len(finger) == 0 {
						return nil, false
					}
					return finger, true
				},
			}

			req := buildStateChangeRequest(tt.method, tt.cookieValue, tt.headerValue, tt.formValue, tt.secFetch)
			rr := httptest.NewRecorder()
			CSRF(opts)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})).ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rr.Code, tt.wantStatus)
			}
		})
	}
}

func TestCSRF_TokenInjectedIntoContext(t *testing.T) {
	t.Parallel()

	finger := []byte("session-token-hash-32-byte-fixed")
	expected := sessionExpected(testCSRFSecret, finger)

	var got string
	handler := CSRF(CSRFOptions{
		Secret: testCSRFSecret,
		GetSessionFinger: func(_ context.Context) ([]byte, bool) {
			return finger, true
		},
	})(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = TokenFromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if got != expected {
		t.Errorf("TokenFromContext = %q, want %q", got, expected)
	}
}

func TestCSRF_CookieIsHttpOnlyAndProperFlags(t *testing.T) {
	t.Parallel()

	for _, secure := range []bool{true, false} {
		t.Run(boolName(secure), func(t *testing.T) {
			t.Parallel()

			handler := CSRF(CSRFOptions{
				Secret:           testCSRFSecret,
				GetSessionFinger: noSession,
				Secure:           secure,
			})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			req.Header.Set("Sec-Fetch-Site", "same-origin")
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			cookies := rr.Result().Cookies()
			var csrfCookie *http.Cookie
			for _, c := range cookies {
				if c.Name == cookieName(secure) {
					csrfCookie = c
					break
				}
			}
			if csrfCookie == nil {
				t.Fatalf("no CSRF cookie set")
			}
			if !csrfCookie.HttpOnly {
				t.Errorf("HttpOnly = false, want true")
			}
			if csrfCookie.Secure != secure {
				t.Errorf("Secure = %v, want %v", csrfCookie.Secure, secure)
			}
			if csrfCookie.Path != "/" {
				t.Errorf("Path = %q, want /", csrfCookie.Path)
			}
			if csrfCookie.SameSite != http.SameSiteLaxMode {
				t.Errorf("SameSite = %v, want Lax", csrfCookie.SameSite)
			}
		})
	}
}

func TestCSRF_AnonymousCookieReusedAcrossRequests(t *testing.T) {
	t.Parallel()

	opts := CSRFOptions{
		Secret:           testCSRFSecret,
		GetSessionFinger: noSession,
	}

	handler := CSRF(opts)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req1 := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req1.Header.Set("Sec-Fetch-Site", "same-origin")
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)

	cookies1 := rr1.Result().Cookies()
	if len(cookies1) == 0 {
		t.Fatalf("no cookie set on first request")
	}
	firstValue := cookies1[0].Value

	req2 := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req2.Header.Set("Sec-Fetch-Site", "same-origin")
	req2.AddCookie(&http.Cookie{Name: cookieName(false), Value: firstValue})
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	for _, c := range rr2.Result().Cookies() {
		if c.Name == cookieName(false) && c.Value != firstValue {
			t.Errorf("cookie rotated unexpectedly: was %q, now %q", firstValue, c.Value)
		}
	}
}

func noSession(_ context.Context) ([]byte, bool) {
	return nil, false
}

func mustNewAnonToken(t *testing.T, secret []byte) string {
	t.Helper()
	tok, err := newAnonymousToken(secret)
	if err != nil {
		t.Fatalf("newAnonymousToken: %v", err)
	}
	return tok
}

func sessionExpected(secret, finger []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(finger)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func buildStateChangeRequest(method, cookieValue, headerValue, formValue, secFetch string) *http.Request {
	var req *http.Request
	if formValue != "" {
		form := url.Values{csrfFormField: []string{formValue}}
		req = httptest.NewRequest(method, "/", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, "/", http.NoBody)
	}
	if headerValue != "" {
		req.Header.Set(csrfHeaderName, headerValue)
	}
	if cookieValue != "" {
		req.AddCookie(&http.Cookie{Name: cookieName(false), Value: cookieValue})
	}
	if secFetch != "" {
		req.Header.Set("Sec-Fetch-Site", secFetch)
	}
	return req
}

func boolName(b bool) string {
	if b {
		return "secure"
	}
	return "insecure"
}

func init() {
	_ = rand.Reader
}
