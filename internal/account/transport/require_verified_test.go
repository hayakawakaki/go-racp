package transport

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hayakawakaki/go-racp/internal/account/domain"
)

func newTestVerifiedMiddleware(t *testing.T, sess *stubSessionService, users *stubUserLookup, allow []string) (func(http.Handler) http.Handler, *bytes.Buffer) {
	t.Helper()
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(buf, nil))
	return RequireVerified(sess, users, logger, allow), buf
}

func TestRequireVerified_AllowlistedPathPassesThrough(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		path  string
		allow []string
	}{
		{name: "exact match", path: "/verify-account", allow: []string{"/verify-account"}},
		{name: "subpath under prefix", path: "/static/app.css", allow: []string{"/static"}},
		{name: "exact prefix root", path: "/login", allow: []string{"/login"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sess := &stubSessionService{}
			users := &stubUserLookup{}
			middleware, _ := newTestVerifiedMiddleware(t, sess, users, tt.allow)

			called := false
			handler := middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				called = true
			}))

			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tt.path, http.NoBody)
			handler.ServeHTTP(rr, req)

			if !called {
				t.Errorf("downstream not called for allowlisted path %q", tt.path)
			}
		})
	}
}

func TestRequireVerified_NoCookie_PassesThrough(t *testing.T) {
	t.Parallel()
	sess := &stubSessionService{}
	users := &stubUserLookup{}
	middleware, _ := newTestVerifiedMiddleware(t, sess, users, nil)

	called := false
	handler := middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/dashboard", http.NoBody)
	handler.ServeHTTP(rr, req)

	if !called {
		t.Errorf("anonymous request must pass through")
	}
	if rr.Code == http.StatusSeeOther {
		t.Errorf("anonymous request should not redirect; got %d", rr.Code)
	}
}

func TestRequireVerified_EmptyCookieValue_PassesThrough(t *testing.T) {
	t.Parallel()
	sess := &stubSessionService{}
	users := &stubUserLookup{}
	middleware, _ := newTestVerifiedMiddleware(t, sess, users, nil)

	called := false
	handler := middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/dashboard", http.NoBody)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: ""})
	handler.ServeHTTP(rr, req)

	if !called {
		t.Errorf("empty cookie should pass through as anonymous")
	}
}

func TestRequireVerified_StaleSessionErrors_PassThrough(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err  error
		name string
	}{
		{name: "not found", err: domain.ErrSessionNotFound},
		{name: "expired", err: domain.ErrSessionExpired},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sess := &stubSessionService{
				validateFn: func(context.Context, string) (*domain.Session, error) {
					return nil, tt.err
				},
			}
			users := &stubUserLookup{}
			middleware, _ := newTestVerifiedMiddleware(t, sess, users, nil)

			called := false
			handler := middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))

			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/dashboard", http.NoBody)
			req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "stale"})
			handler.ServeHTTP(rr, req)

			if !called {
				t.Errorf("stale session should pass through (handled by RequireAuth/WithSession)")
			}
		})
	}
}

func TestRequireVerified_GenericSessionError_LogsAndPassesThrough(t *testing.T) {
	t.Parallel()
	sess := &stubSessionService{
		validateFn: func(context.Context, string) (*domain.Session, error) {
			return nil, errors.New("db unreachable")
		},
	}
	users := &stubUserLookup{}
	middleware, logBuffer := newTestVerifiedMiddleware(t, sess, users, nil)

	called := false
	handler := middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/dashboard", http.NoBody)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "x"})
	handler.ServeHTTP(rr, req)

	if !called {
		t.Errorf("opaque session error should fail open and pass through")
	}
	if !strings.Contains(logBuffer.String(), "require_verified: session validate") {
		t.Errorf("expected error log, got %q", logBuffer.String())
	}
	if !strings.Contains(logBuffer.String(), "db unreachable") {
		t.Errorf("error message not propagated to log: %q", logBuffer.String())
	}
}

func TestRequireVerified_UserLookupError_LogsAndPassesThrough(t *testing.T) {
	t.Parallel()
	sess := &stubSessionService{
		validateFn: func(context.Context, string) (*domain.Session, error) {
			return &domain.Session{UserID: 1}, nil
		},
	}
	users := &stubUserLookup{
		getByIDFn: func(context.Context, int) (*domain.User, error) {
			return nil, errors.New("user lookup boom")
		},
	}
	middleware, logBuffer := newTestVerifiedMiddleware(t, sess, users, nil)

	called := false
	handler := middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/dashboard", http.NoBody)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "ok"})
	handler.ServeHTTP(rr, req)

	if !called {
		t.Errorf("user lookup failure should fail open and pass through")
	}
	if !strings.Contains(logBuffer.String(), "require_verified: load user") {
		t.Errorf("expected user-lookup error log, got %q", logBuffer.String())
	}
}

func TestRequireVerified_UnverifiedUser_RedirectsToVerifyAccount(t *testing.T) {
	t.Parallel()
	sess := &stubSessionService{
		validateFn: func(context.Context, string) (*domain.Session, error) {
			return &domain.Session{UserID: 1}, nil
		},
	}
	users := &stubUserLookup{
		getByIDFn: func(_ context.Context, id int) (*domain.User, error) {
			return &domain.User{ID: id, GroupID: 5}, nil
		},
	}
	middleware, _ := newTestVerifiedMiddleware(t, sess, users, nil)

	called := false
	handler := middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/dashboard", http.NoBody)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "ok"})
	handler.ServeHTTP(rr, req)

	if called {
		t.Errorf("downstream should not run for unverified user")
	}
	if rr.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusSeeOther)
	}
	if rr.Header().Get("Location") != "/verify-account" {
		t.Errorf("Location = %q, want /verify-account", rr.Header().Get("Location"))
	}
}

func TestRequireVerified_VerifiedUser_PassesThrough(t *testing.T) {
	t.Parallel()
	sess := &stubSessionService{
		validateFn: func(context.Context, string) (*domain.Session, error) {
			return &domain.Session{UserID: 1}, nil
		},
	}
	users := &stubUserLookup{
		getByIDFn: func(_ context.Context, id int) (*domain.User, error) {
			return &domain.User{ID: id, GroupID: 0}, nil
		},
	}
	middleware, _ := newTestVerifiedMiddleware(t, sess, users, nil)

	called := false
	handler := middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/dashboard", http.NoBody)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "ok"})
	handler.ServeHTTP(rr, req)

	if !called {
		t.Errorf("verified user should reach downstream")
	}
	if rr.Code == http.StatusSeeOther {
		t.Errorf("verified user must not be redirected; got %d, Location=%q", rr.Code, rr.Header().Get("Location"))
	}
}

func TestRequireVerified_AllowlistShortCircuitsBeforeSessionLookup(t *testing.T) {
	t.Parallel()
	validateCalled := false
	sess := &stubSessionService{
		validateFn: func(context.Context, string) (*domain.Session, error) {
			validateCalled = true
			return &domain.Session{UserID: 1}, nil
		},
	}
	users := &stubUserLookup{
		getByIDFn: func(_ context.Context, id int) (*domain.User, error) {
			return &domain.User{ID: id, GroupID: 5}, nil
		},
	}
	middleware, _ := newTestVerifiedMiddleware(t, sess, users, []string{"/verify-account"})

	called := false
	handler := middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/verify-account", http.NoBody)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "ok"})
	handler.ServeHTTP(rr, req)

	if !called {
		t.Errorf("allowlisted path must pass through even for unverified users")
	}
	if validateCalled {
		t.Errorf("allowlisted path should short-circuit before session validation")
	}
}

func TestIsAllowed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		path     string
		prefixes []string
		want     bool
	}{
		{name: "empty prefixes never allows", path: "/x", prefixes: nil, want: false},
		{name: "exact match", path: "/login", prefixes: []string{"/login"}, want: true},
		{name: "subpath under prefix", path: "/static/css/app.css", prefixes: []string{"/static"}, want: true},
		{name: "prefix collision rejected", path: "/loginx", prefixes: []string{"/login"}, want: false},
		{name: "second prefix matches", path: "/healthz", prefixes: []string{"/login", "/healthz"}, want: true},
		{name: "no match", path: "/admin", prefixes: []string{"/login", "/healthz"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isAllowed(tt.path, tt.prefixes); got != tt.want {
				t.Errorf("isAllowed(%q, %v) = %v, want %v", tt.path, tt.prefixes, got, tt.want)
			}
		})
	}
}
