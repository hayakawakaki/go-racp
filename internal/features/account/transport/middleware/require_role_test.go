package middleware

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
	"github.com/hayakawakaki/go-racp/internal/httpx"
	"github.com/hayakawakaki/go-racp/server/config"
)

func newTestRoleResolver() domain.RoleResolver {
	return domain.NewRoleResolver(config.RolesConfig{
		"Moderator": 20,
		"Enforcer":  10,
		"Event":     2,
	})
}

var (
	testRoleModerator = domain.Role{Name: "Moderator", GroupID: 20}
	testRoleEvent     = domain.Role{Name: "Event", GroupID: 2}
)

func newTestRoleMiddleware(sess *stubSessionService, users *stubUserLookup, hidden bool, allowed ...domain.Role) (func(http.Handler) http.Handler, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(buf, nil))
	resolver := newTestRoleResolver()
	policy := AuthPolicy{AllowTempBannedLogin: true}
	if hidden {
		return RequireRoleHidden(sess, users, resolver, logger, false, httpx.Layout{}, policy, allowed...), buf
	}
	return RequireRole(sess, users, resolver, logger, false, policy, allowed...), buf
}

func userWithGroup(groupID int) func(context.Context, int) (*domain.User, error) {
	return func(_ context.Context, id int) (*domain.User, error) {
		return &domain.User{ID: id, GroupID: groupID}, nil
	}
}

func TestRequireRole_NoCookie_RedirectsToLogin(t *testing.T) {
	t.Parallel()
	middleware, _ := newTestRoleMiddleware(&stubSessionService{}, &stubUserLookup{}, false, domain.RoleAuthenticated)

	called := false
	handler := middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", http.NoBody)
	handler.ServeHTTP(rr, req)

	if called {
		t.Errorf("downstream must not be called without a cookie")
	}
	if rr.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusSeeOther)
	}
	if rr.Header().Get("Location") != "/login" {
		t.Errorf("Location = %q, want /login", rr.Header().Get("Location"))
	}
}

func TestRequireRole_NoCookie_HTMXSendsHXRedirect(t *testing.T) {
	t.Parallel()
	middleware, _ := newTestRoleMiddleware(&stubSessionService{}, &stubUserLookup{}, false, domain.RoleAuthenticated)

	handler := middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", http.NoBody)
	req.Header.Set("HX-Request", "true")
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
	if rr.Header().Get("HX-Redirect") != "/login" {
		t.Errorf("HX-Redirect = %q, want /login", rr.Header().Get("HX-Redirect"))
	}
}

func TestRequireRole_NoCookie_HiddenReturns404(t *testing.T) {
	t.Parallel()
	middleware, _ := newTestRoleMiddleware(&stubSessionService{}, &stubUserLookup{}, true, domain.RoleAdmin)

	called := false
	handler := middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", http.NoBody)
	handler.ServeHTTP(rr, req)

	if called {
		t.Errorf("downstream must not be called without a cookie")
	}
	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
	if rr.Header().Get("Location") != "" {
		t.Errorf("hidden mode must not send Location, got %q", rr.Header().Get("Location"))
	}
}

func TestRequireRole_EmptyCookieValue_RedirectsToLogin(t *testing.T) {
	t.Parallel()
	middleware, _ := newTestRoleMiddleware(&stubSessionService{}, &stubUserLookup{}, false, domain.RoleAuthenticated)

	handler := middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", http.NoBody)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: ""})
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("Location") != "/login" {
		t.Errorf("Location = %q, want /login", rr.Header().Get("Location"))
	}
}

func TestRequireRole_StaleSession_ClearsCookieAndRedirects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err  error
		name string
	}{
		{name: "session not found", err: domain.ErrSessionNotFound},
		{name: "session expired", err: domain.ErrSessionExpired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sess := &stubSessionService{
				validateFn: func(context.Context, string) (*domain.Session, error) { return nil, tt.err },
			}
			middleware, _ := newTestRoleMiddleware(sess, &stubUserLookup{}, false, domain.RoleAuthenticated)

			called := false
			handler := middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))

			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/admin", http.NoBody)
			req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "stale"})
			handler.ServeHTTP(rr, req)

			if called {
				t.Errorf("downstream must not be called for stale session")
			}
			if rr.Header().Get("Location") != "/login" {
				t.Errorf("Location = %q, want /login", rr.Header().Get("Location"))
			}
			cookie := findSetCookie(rr, SessionCookieName)
			if cookie == nil {
				t.Fatalf("expected Set-Cookie clearing %s", SessionCookieName)
			}
			if cookie.MaxAge >= 0 {
				t.Errorf("cookie.MaxAge = %d, want < 0 to clear", cookie.MaxAge)
			}
		})
	}
}

func TestRequireRole_StaleSession_HiddenClearsCookieAndReturns404(t *testing.T) {
	t.Parallel()
	sess := &stubSessionService{
		validateFn: func(context.Context, string) (*domain.Session, error) { return nil, domain.ErrSessionExpired },
	}
	middleware, _ := newTestRoleMiddleware(sess, &stubUserLookup{}, true, domain.RoleAdmin)

	handler := middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", http.NoBody)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "stale"})
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
	cookie := findSetCookie(rr, SessionCookieName)
	if cookie == nil || cookie.MaxAge >= 0 {
		t.Errorf("expected stale cookie cleared, got %+v", cookie)
	}
}

func TestRequireRole_GenericSessionError_Returns500AndLogs(t *testing.T) {
	t.Parallel()
	sess := &stubSessionService{
		validateFn: func(context.Context, string) (*domain.Session, error) {
			return nil, errors.New("db unreachable")
		},
	}
	middleware, logBuffer := newTestRoleMiddleware(sess, &stubUserLookup{}, false, domain.RoleAuthenticated)

	called := false
	handler := middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", http.NoBody)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "x"})
	handler.ServeHTTP(rr, req)

	if called {
		t.Errorf("downstream must not be called on opaque error")
	}
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
	if !strings.Contains(logBuffer.String(), "require_role: session validate") {
		t.Errorf("expected require_role error log, got %q", logBuffer.String())
	}
}

func TestRequireRole_UserLookupError_Returns500AndLogs(t *testing.T) {
	t.Parallel()
	sess := &stubSessionService{
		validateFn: func(context.Context, string) (*domain.Session, error) {
			return &domain.Session{UserID: 7}, nil
		},
	}
	users := &stubUserLookup{
		getByIDFn: func(context.Context, int) (*domain.User, error) {
			return nil, errors.New("user lookup boom")
		},
	}
	middleware, logBuffer := newTestRoleMiddleware(sess, users, false, domain.RoleAuthenticated)

	called := false
	handler := middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", http.NoBody)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "ok"})
	handler.ServeHTTP(rr, req)

	if called {
		t.Errorf("downstream must not be called on user lookup failure")
	}
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
	if !strings.Contains(logBuffer.String(), "require_role: load user") {
		t.Errorf("expected user lookup error log, got %q", logBuffer.String())
	}
}

func TestRequireRole_AllowedRole_PassesThroughWithSessionInContext(t *testing.T) {
	t.Parallel()
	wantSession := &domain.Session{UserID: 42}
	sess := &stubSessionService{
		validateFn: func(context.Context, string) (*domain.Session, error) { return wantSession, nil },
	}
	users := &stubUserLookup{getByIDFn: userWithGroup(20)}
	middleware, _ := newTestRoleMiddleware(sess, users, false, testRoleModerator)

	var gotSession *domain.Session
	var ok bool
	handler := middleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotSession, ok = SessionFromContext(r.Context())
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", http.NoBody)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "ok"})
	handler.ServeHTTP(rr, req)

	if !ok {
		t.Fatalf("session not attached to context")
	}
	if gotSession.UserID != wantSession.UserID {
		t.Errorf("UserID = %d, want %d", gotSession.UserID, wantSession.UserID)
	}
}

func TestRequireRole_AdminAlwaysAllowed(t *testing.T) {
	t.Parallel()
	sess := &stubSessionService{
		validateFn: func(context.Context, string) (*domain.Session, error) {
			return &domain.Session{UserID: 1}, nil
		},
	}
	users := &stubUserLookup{getByIDFn: userWithGroup(99)}
	middleware, _ := newTestRoleMiddleware(sess, users, false, testRoleEvent)

	called := false
	handler := middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", http.NoBody)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "ok"})
	handler.ServeHTTP(rr, req)

	if !called {
		t.Errorf("admin must pass through even when not in allowed list")
	}
}

func TestRequireRole_RoleAuthenticatedAllowsAllSessions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		groupID int
	}{
		{name: "player", groupID: 0},
		{name: "event", groupID: 2},
		{name: "moderator", groupID: 20},
		{name: "enforcer", groupID: 10},
		{name: "admin", groupID: 99},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sess := &stubSessionService{
				validateFn: func(context.Context, string) (*domain.Session, error) {
					return &domain.Session{UserID: 1}, nil
				},
			}
			users := &stubUserLookup{getByIDFn: userWithGroup(tt.groupID)}
			middleware, _ := newTestRoleMiddleware(sess, users, false, domain.RoleAuthenticated)

			called := false
			handler := middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))

			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/account", http.NoBody)
			req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "ok"})
			handler.ServeHTTP(rr, req)

			if !called {
				t.Errorf("RoleAuthenticated must allow groupID=%d through", tt.groupID)
			}
		})
	}
}

func TestRequireRole_DisallowedRole_Forbidden(t *testing.T) {
	t.Parallel()
	sess := &stubSessionService{
		validateFn: func(context.Context, string) (*domain.Session, error) {
			return &domain.Session{UserID: 1}, nil
		},
	}
	users := &stubUserLookup{getByIDFn: userWithGroup(0)}
	middleware, _ := newTestRoleMiddleware(sess, users, false, testRoleModerator)

	called := false
	handler := middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", http.NoBody)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "ok"})
	handler.ServeHTTP(rr, req)

	if called {
		t.Errorf("downstream must not be called when role disallowed")
	}
	if rr.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusForbidden)
	}
}

func TestRequireRole_DisallowedRole_HiddenReturns404(t *testing.T) {
	t.Parallel()
	sess := &stubSessionService{
		validateFn: func(context.Context, string) (*domain.Session, error) {
			return &domain.Session{UserID: 1}, nil
		},
	}
	users := &stubUserLookup{getByIDFn: userWithGroup(0)}
	middleware, _ := newTestRoleMiddleware(sess, users, true, domain.RoleAdmin)

	called := false
	handler := middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", http.NoBody)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "ok"})
	handler.ServeHTTP(rr, req)

	if called {
		t.Errorf("downstream must not be called when role disallowed")
	}
	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d (hidden mode masks forbidden as 404)", rr.Code, http.StatusNotFound)
	}
}
