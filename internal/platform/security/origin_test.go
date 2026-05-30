package security

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOrigin_SafeMethodsPassThrough(t *testing.T) {
	t.Parallel()

	methods := []string{http.MethodGet, http.MethodHead, http.MethodOptions}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			t.Parallel()

			called := false
			mw := Origin(OriginOptions{})
			handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(method, "/", http.NoBody)
			req.Host = "panel.example"
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if !called {
				t.Errorf("next handler not invoked for %s", method)
			}
			if rr.Code != http.StatusOK {
				t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
			}
		})
	}
}

func TestOrigin_StateChangingMethods(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		method         string
		host           string
		originHeader   string
		refererHeader  string
		secFetchSite   string
		trustedOrigins []string
		wantStatus     int
	}{
		{
			name:         "POST with matching Origin passes",
			method:       http.MethodPost,
			host:         "panel.example",
			originHeader: "https://panel.example",
			wantStatus:   http.StatusOK,
		},
		{
			name:         "POST with mismatched Origin rejected",
			method:       http.MethodPost,
			host:         "panel.example",
			originHeader: "https://attacker.example",
			wantStatus:   http.StatusForbidden,
		},
		{
			name:          "POST with matching Referer passes when Origin absent",
			method:        http.MethodPost,
			host:          "panel.example",
			refererHeader: "https://panel.example/login",
			wantStatus:    http.StatusOK,
		},
		{
			name:          "POST with mismatched Referer rejected",
			method:        http.MethodPost,
			host:          "panel.example",
			refererHeader: "https://attacker.example/login",
			wantStatus:    http.StatusForbidden,
		},
		{
			name:       "POST with no Origin or Referer rejected",
			method:     http.MethodPost,
			host:       "panel.example",
			wantStatus: http.StatusForbidden,
		},
		{
			name:           "POST with trusted Origin passes",
			method:         http.MethodPost,
			host:           "panel.example",
			originHeader:   "https://bot.example",
			trustedOrigins: []string{"https://bot.example"},
			wantStatus:     http.StatusOK,
		},
		{
			name:           "trusted entry with trailing slash still matches",
			method:         http.MethodPost,
			host:           "panel.example",
			originHeader:   "https://bot.example",
			trustedOrigins: []string{"https://bot.example/"},
			wantStatus:     http.StatusOK,
		},
		{
			name:           "trusted entry with uppercase scheme still matches",
			method:         http.MethodPost,
			host:           "panel.example",
			originHeader:   "https://bot.example",
			trustedOrigins: []string{"HTTPS://BOT.EXAMPLE"},
			wantStatus:     http.StatusOK,
		},
		{
			name:         "Origin with port matches host with port",
			method:       http.MethodPost,
			host:         "panel.example:8080",
			originHeader: "https://panel.example:8080",
			wantStatus:   http.StatusOK,
		},
		{
			name:         "case-different host matches",
			method:       http.MethodPost,
			host:         "PANEL.example",
			originHeader: "https://panel.EXAMPLE",
			wantStatus:   http.StatusOK,
		},
		{
			name:         "PUT with mismatched Origin rejected",
			method:       http.MethodPut,
			host:         "panel.example",
			originHeader: "https://attacker.example",
			wantStatus:   http.StatusForbidden,
		},
		{
			name:         "DELETE with matching Origin passes",
			method:       http.MethodDelete,
			host:         "panel.example",
			originHeader: "https://panel.example",
			wantStatus:   http.StatusOK,
		},
		{
			name:         "PATCH with matching Origin passes",
			method:       http.MethodPatch,
			host:         "panel.example",
			originHeader: "https://panel.example",
			wantStatus:   http.StatusOK,
		},
		{
			name:         "malformed Origin rejected",
			method:       http.MethodPost,
			host:         "panel.example",
			originHeader: "::::not-a-url",
			wantStatus:   http.StatusForbidden,
		},
		{
			name:         "Origin with empty host rejected",
			method:       http.MethodPost,
			host:         "panel.example",
			originHeader: "https://",
			wantStatus:   http.StatusForbidden,
		},
		{
			name:         "Sec-Fetch-Site same-origin short-circuits with null Origin",
			method:       http.MethodPost,
			host:         "panel.example",
			originHeader: "null",
			secFetchSite: "same-origin",
			wantStatus:   http.StatusOK,
		},
		{
			name:         "Sec-Fetch-Site same-origin short-circuits with no Origin or Referer",
			method:       http.MethodPost,
			host:         "panel.example",
			secFetchSite: "same-origin",
			wantStatus:   http.StatusOK,
		},
		{
			name:         "Sec-Fetch-Site cross-site does not short-circuit",
			method:       http.MethodPost,
			host:         "panel.example",
			originHeader: "https://attacker.example",
			secFetchSite: "cross-site",
			wantStatus:   http.StatusForbidden,
		},
		{
			name:         "Sec-Fetch-Site same-site does not short-circuit",
			method:       http.MethodPost,
			host:         "panel.example",
			originHeader: "https://sub.attacker.example",
			secFetchSite: "same-site",
			wantStatus:   http.StatusForbidden,
		},
		{
			name:         "null Origin without Sec-Fetch-Site rejected",
			method:       http.MethodPost,
			host:         "panel.example",
			originHeader: "null",
			wantStatus:   http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mw := Origin(OriginOptions{TrustedOrigins: tt.trustedOrigins})
			handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(tt.method, "/", http.NoBody)
			req.Host = tt.host
			if tt.originHeader != "" {
				req.Header.Set("Origin", tt.originHeader)
			}
			if tt.refererHeader != "" {
				req.Header.Set("Referer", tt.refererHeader)
			}
			if tt.secFetchSite != "" {
				req.Header.Set("Sec-Fetch-Site", tt.secFetchSite)
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rr.Code, tt.wantStatus)
			}
		})
	}
}

func TestOrigin_OpenRoutesBypass(t *testing.T) {
	t.Parallel()

	matcher, err := NewRouteMatcher([]string{"/webhooks/*"})
	if err != nil {
		t.Fatalf("NewRouteMatcher: %v", err)
	}

	tests := []struct {
		name       string
		target     string
		wantStatus int
		wantNext   bool
	}{
		{
			name:       "open route bypasses origin check",
			target:     "/webhooks/stripe",
			wantStatus: http.StatusTeapot,
			wantNext:   true,
		},
		{
			name:       "non-open path still rejected",
			target:     "/account/password",
			wantStatus: http.StatusForbidden,
			wantNext:   false,
		},
		{
			name:       "traversal into non-open path rejected",
			target:     "/webhooks/../account/password",
			wantStatus: http.StatusForbidden,
			wantNext:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			called := false
			mw := Origin(OriginOptions{OpenRoutes: matcher})
			handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusTeapot)
			}))

			req := httptest.NewRequest(http.MethodPost, tt.target, http.NoBody)
			req.Host = "panel.example"
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if called != tt.wantNext {
				t.Errorf("next called = %v, want %v", called, tt.wantNext)
			}
			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rr.Code, tt.wantStatus)
			}
		})
	}
}

func TestOrigin_RejectionDoesNotInvokeNext(t *testing.T) {
	t.Parallel()

	called := false
	mw := Origin(OriginOptions{})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	req.Host = "panel.example"
	req.Header.Set("Origin", "https://attacker.example")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if called {
		t.Errorf("next handler invoked despite Origin mismatch")
	}
	if rr.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusForbidden)
	}
}
