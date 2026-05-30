package security

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hayakawakaki/go-racp/server/config"
)

func TestHeaders_AlwaysSetsBaselineHeaders(t *testing.T) {
	t.Parallel()

	rr := runHeaders(t, HeadersOptions{Cfg: config.SecurityConfig{HSTSMaxAge: 31536000}, Secure: true})

	expected := map[string]string{
		"X-Content-Type-Options":       "nosniff",
		"X-Frame-Options":              "DENY",
		"Referrer-Policy":              "strict-origin-when-cross-origin",
		"Permissions-Policy":           "camera=(), microphone=(), geolocation=(), interest-cohort=()",
		"Cross-Origin-Opener-Policy":   "same-origin",
		"Cross-Origin-Resource-Policy": "same-origin",
	}
	for k, want := range expected {
		if got := rr.Header().Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	if got := rr.Header().Get("Content-Security-Policy"); got == "" {
		t.Errorf("Content-Security-Policy missing")
	}
}

func TestHeaders_HSTS(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		want   string
		cfg    config.SecurityConfig
		secure bool
	}{
		{
			name:   "insecure suppresses HSTS",
			secure: false,
			cfg:    config.SecurityConfig{HSTSMaxAge: 31536000, HSTSIncludeSubdomains: true},
			want:   "",
		},
		{
			name:   "zero max-age suppresses HSTS",
			secure: true,
			cfg:    config.SecurityConfig{HSTSMaxAge: 0, HSTSIncludeSubdomains: true},
			want:   "",
		},
		{
			name:   "negative max-age suppresses HSTS",
			secure: true,
			cfg:    config.SecurityConfig{HSTSMaxAge: -1},
			want:   "",
		},
		{
			name:   "bare max-age",
			secure: true,
			cfg:    config.SecurityConfig{HSTSMaxAge: 600},
			want:   "max-age=600",
		},
		{
			name:   "with includeSubDomains only",
			secure: true,
			cfg:    config.SecurityConfig{HSTSMaxAge: 700, HSTSIncludeSubdomains: true},
			want:   "max-age=700; includeSubDomains",
		},
		{
			name:   "with preload only",
			secure: true,
			cfg:    config.SecurityConfig{HSTSMaxAge: 800, HSTSPreload: true},
			want:   "max-age=800; preload",
		},
		{
			name:   "full directives",
			secure: true,
			cfg:    config.SecurityConfig{HSTSMaxAge: 900, HSTSIncludeSubdomains: true, HSTSPreload: true},
			want:   "max-age=900; includeSubDomains; preload",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rr := runHeaders(t, HeadersOptions{Cfg: tt.cfg, Secure: tt.secure})
			if got := rr.Header().Get("Strict-Transport-Security"); got != tt.want {
				t.Errorf("HSTS = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHeaders_CSPDirectives(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want []string
		cfg  config.SecurityConfig
	}{
		{
			name: "baseline directives present",
			cfg:  config.SecurityConfig{},
			want: []string{
				"default-src 'self'",
				"script-src 'self'",
				"style-src 'self' 'unsafe-inline'",
				"img-src 'self' data:",
				"font-src 'self' data:",
				"connect-src 'self'",
				"frame-ancestors 'none'",
				"form-action 'self'",
				"base-uri 'self'",
				"object-src 'none'",
				"upgrade-insecure-requests",
			},
		},
		{
			name: "extra script source appended",
			cfg:  config.SecurityConfig{CSPExtraScriptSrc: []string{"https://cdn.example.com"}},
			want: []string{"script-src 'self' https://cdn.example.com"},
		},
		{
			name: "extra style source appended",
			cfg:  config.SecurityConfig{CSPExtraStyleSrc: []string{"https://fonts.googleapis.com"}},
			want: []string{"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com"},
		},
		{
			name: "extra img source appended",
			cfg:  config.SecurityConfig{CSPExtraImgSrc: []string{"https://i.imgur.com"}},
			want: []string{"img-src 'self' data: https://i.imgur.com"},
		},
		{
			name: "extra form-action source appended",
			cfg:  config.SecurityConfig{CSPExtraFormAction: []string{"https://checkout.stripe.com"}},
			want: []string{"form-action 'self' https://checkout.stripe.com"},
		},
		{
			name: "multiple extras combine",
			cfg: config.SecurityConfig{
				CSPExtraScriptSrc: []string{"https://a.example", "https://b.example"},
				CSPExtraImgSrc:    []string{"https://i.imgur.com", "https://cdn.example.org"},
			},
			want: []string{
				"script-src 'self' https://a.example https://b.example",
				"img-src 'self' data: https://i.imgur.com https://cdn.example.org",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rr := runHeaders(t, HeadersOptions{Cfg: tt.cfg, Secure: true})
			csp := rr.Header().Get("Content-Security-Policy")
			for _, directive := range tt.want {
				if !strings.Contains(csp, directive) {
					t.Errorf("CSP missing directive %q\nfull CSP: %s", directive, csp)
				}
			}
		})
	}
}

func TestHeaders_DoesNotMutateConfigSlices(t *testing.T) {
	t.Parallel()

	cfg := config.SecurityConfig{
		CSPExtraScriptSrc:  []string{"https://a.example"},
		CSPExtraStyleSrc:   []string{"https://b.example"},
		CSPExtraImgSrc:     []string{"https://i.imgur.com"},
		CSPExtraFormAction: []string{"https://checkout.stripe.com"},
	}
	scriptBefore := append([]string(nil), cfg.CSPExtraScriptSrc...)
	styleBefore := append([]string(nil), cfg.CSPExtraStyleSrc...)
	imgBefore := append([]string(nil), cfg.CSPExtraImgSrc...)
	formActionBefore := append([]string(nil), cfg.CSPExtraFormAction...)

	_ = runHeaders(t, HeadersOptions{Cfg: cfg, Secure: true})

	if !equalStrings(cfg.CSPExtraScriptSrc, scriptBefore) {
		t.Errorf("CSPExtraScriptSrc mutated: got %v, want %v", cfg.CSPExtraScriptSrc, scriptBefore)
	}
	if !equalStrings(cfg.CSPExtraStyleSrc, styleBefore) {
		t.Errorf("CSPExtraStyleSrc mutated: got %v, want %v", cfg.CSPExtraStyleSrc, styleBefore)
	}
	if !equalStrings(cfg.CSPExtraImgSrc, imgBefore) {
		t.Errorf("CSPExtraImgSrc mutated: got %v, want %v", cfg.CSPExtraImgSrc, imgBefore)
	}
	if !equalStrings(cfg.CSPExtraFormAction, formActionBefore) {
		t.Errorf("CSPExtraFormAction mutated: got %v, want %v", cfg.CSPExtraFormAction, formActionBefore)
	}
}

func TestHeaders_CallsNextHandler(t *testing.T) {
	t.Parallel()

	called := false
	mw := Headers(HeadersOptions{Secure: true})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusTeapot)
	}))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", http.NoBody))

	if !called {
		t.Errorf("next handler not invoked")
	}
	if rr.Code != http.StatusTeapot {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusTeapot)
	}
}

func runHeaders(t *testing.T, opts HeadersOptions) *httptest.ResponseRecorder {
	t.Helper()
	mw := Headers(opts)
	rr := httptest.NewRecorder()
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", http.NoBody))
	return rr
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
