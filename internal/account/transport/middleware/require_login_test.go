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

	"github.com/hayakawakaki/go-racp/internal/account/domain"
)

func newRequireLogin(sess *stubSessionService, logBuffer *bytes.Buffer) func(http.Handler) http.Handler {
	logger := slog.New(slog.NewTextHandler(logBuffer, nil))
	return RequireLogin(sess, logger)
}

func TestRequireLogin_NoCookie_RedirectsToLogin(t *testing.T) {
	t.Parallel()
	middleware := newRequireLogin(&stubSessionService{}, &bytes.Buffer{})

	called := false
	handler := middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/account", http.NoBody)
	handler.ServeHTTP(rr, req)

	if called {
		t.Errorf("downstream must not be called without a cookie")
	}
	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/login" {
		t.Errorf("status = %d, Location = %q; want 303 to /login", rr.Code, rr.Header().Get("Location"))
	}
}

func TestRequireLogin_EmptyCookie_RedirectsToLogin(t *testing.T) {
	t.Parallel()
	middleware := newRequireLogin(&stubSessionService{}, &bytes.Buffer{})

	handler := middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/account", http.NoBody)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: ""})
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("Location") != "/login" {
		t.Errorf("Location = %q, want /login", rr.Header().Get("Location"))
	}
}

func TestRequireLogin_StaleSession_RedirectsToLogin(t *testing.T) {
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
				validateFn: func(context.Context, string) (*domain.Session, error) { return nil, tt.err },
			}
			middleware := newRequireLogin(sess, &bytes.Buffer{})

			called := false
			handler := middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))

			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/account", http.NoBody)
			req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "stale"})
			handler.ServeHTTP(rr, req)

			if called {
				t.Errorf("downstream must not be called for stale session")
			}
			if rr.Header().Get("Location") != "/login" {
				t.Errorf("Location = %q, want /login", rr.Header().Get("Location"))
			}
		})
	}
}

func TestRequireLogin_GenericError_ReturnsInternalErrorAndLogs(t *testing.T) {
	t.Parallel()
	logBuffer := &bytes.Buffer{}
	sess := &stubSessionService{
		validateFn: func(context.Context, string) (*domain.Session, error) {
			return nil, errors.New("db unreachable")
		},
	}
	middleware := newRequireLogin(sess, logBuffer)

	called := false
	handler := middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/account", http.NoBody)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "x"})
	handler.ServeHTTP(rr, req)

	if called {
		t.Errorf("downstream must not be called on opaque error")
	}
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
	if !strings.Contains(logBuffer.String(), "require_login") {
		t.Errorf("expected require_login in log; got %q", logBuffer.String())
	}
}

func TestRequireLogin_ValidSession_AttachesSessionAndPassesThrough(t *testing.T) {
	t.Parallel()
	want := &domain.Session{UserID: 42}
	sess := &stubSessionService{
		validateFn: func(context.Context, string) (*domain.Session, error) { return want, nil },
	}
	middleware := newRequireLogin(sess, &bytes.Buffer{})

	var gotSess *domain.Session
	var ok bool
	handler := middleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotSess, ok = SessionFromContext(r.Context())
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/account", http.NoBody)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "ok"})
	handler.ServeHTTP(rr, req)

	if !ok {
		t.Fatalf("session not attached to context")
	}
	if gotSess.UserID != want.UserID {
		t.Errorf("UserID = %d, want %d", gotSess.UserID, want.UserID)
	}
}
