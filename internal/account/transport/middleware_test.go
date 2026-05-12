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
	"time"

	"github.com/hayakawakaki/go-racp/internal/account/domain"
)

func mwTestRig(sess *stubSessionService, secure bool) (*stubSessionService, *slog.Logger, bool, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(buf, nil))
	return sess, logger, secure, buf
}

func TestRequireAuth_NoCookie(t *testing.T) {
	t.Parallel()
	sess, logger, secure, _ := mwTestRig(&stubSessionService{}, false)

	called := false
	wrapped := RequireAuth(sess, logger, secure)(func(http.ResponseWriter, *http.Request) { called = true })

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/profile", http.NoBody)
	wrapped(rr, req)

	if called {
		t.Errorf("downstream should not be called")
	}
	if rr.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusSeeOther)
	}
	if rr.Header().Get("Location") != "/login" {
		t.Errorf("Location = %q, want /login", rr.Header().Get("Location"))
	}
}

func TestRequireAuth_NoCookie_HTMX(t *testing.T) {
	t.Parallel()
	sess, logger, secure, _ := mwTestRig(&stubSessionService{}, false)

	wrapped := RequireAuth(sess, logger, secure)(func(http.ResponseWriter, *http.Request) {})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/profile", http.NoBody)
	req.Header.Set("HX-Request", "true")
	wrapped(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
	if rr.Header().Get("HX-Redirect") != "/login" {
		t.Errorf("HX-Redirect = %q, want /login", rr.Header().Get("HX-Redirect"))
	}
}

func TestRequireAuth_ValidSession(t *testing.T) {
	t.Parallel()
	want := &domain.Session{UserID: 42, ExpiresAt: time.Now().Add(time.Hour)}
	sess, logger, secure, _ := mwTestRig(&stubSessionService{
		validateFn: func(context.Context, string) (*domain.Session, error) { return want, nil },
	}, false)

	var gotSess *domain.Session
	var gotOk bool
	wrapped := RequireAuth(sess, logger, secure)(func(_ http.ResponseWriter, r *http.Request) {
		gotSess, gotOk = SessionFromContext(r.Context())
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/profile", http.NoBody)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "any-token"})
	wrapped(rr, req)

	if !gotOk {
		t.Fatalf("SessionFromContext returned !ok")
	}
	if gotSess.UserID != want.UserID {
		t.Errorf("UserID = %d, want %d", gotSess.UserID, want.UserID)
	}
}

func TestRequireAuth_ExpiredOrNotFound_ClearsCookie(t *testing.T) {
	t.Parallel()
	cases := []struct {
		err  error
		name string
	}{
		{name: "expired", err: domain.ErrSessionExpired},
		{name: "not found", err: domain.ErrSessionNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sess, logger, secure, _ := mwTestRig(&stubSessionService{
				validateFn: func(context.Context, string) (*domain.Session, error) { return nil, tc.err },
			}, false)
			wrapped := RequireAuth(sess, logger, secure)(func(http.ResponseWriter, *http.Request) {})

			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/profile", http.NoBody)
			req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "stale"})
			wrapped(rr, req)

			if rr.Code != http.StatusSeeOther {
				t.Errorf("status = %d, want %d", rr.Code, http.StatusSeeOther)
			}
			cookie := findSetCookie(rr, sessionCookieName)
			if cookie == nil {
				t.Fatalf("expected Set-Cookie clearing %s", sessionCookieName)
			}
			if cookie.MaxAge >= 0 {
				t.Errorf("cookie.MaxAge = %d, want < 0 to clear", cookie.MaxAge)
			}
		})
	}
}

func TestRequireAuth_GenericError_Logged(t *testing.T) {
	t.Parallel()
	sess, logger, secure, buf := mwTestRig(&stubSessionService{
		validateFn: func(context.Context, string) (*domain.Session, error) {
			return nil, errors.New("db unreachable")
		},
	}, false)
	wrapped := RequireAuth(sess, logger, secure)(func(http.ResponseWriter, *http.Request) {})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/profile", http.NoBody)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "x"})
	wrapped(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusSeeOther)
	}
	if !strings.Contains(buf.String(), "db unreachable") {
		t.Errorf("expected error logged, got: %q", buf.String())
	}
}

func TestWithSession_NoCookie_PassesThroughAnonymous(t *testing.T) {
	t.Parallel()
	sess, logger, secure, _ := mwTestRig(&stubSessionService{}, false)

	called := false
	var ctxHadSession bool
	wrapped := WithSession(sess, logger, secure)(func(_ http.ResponseWriter, r *http.Request) {
		called = true
		_, ctxHadSession = SessionFromContext(r.Context())
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	wrapped(rr, req)

	if !called {
		t.Errorf("downstream not called for anonymous request")
	}
	if ctxHadSession {
		t.Errorf("context should not contain a session for anonymous request")
	}
	if findSetCookie(rr, sessionCookieName) != nil {
		t.Errorf("anonymous request should not set Set-Cookie")
	}
}

func TestWithSession_ValidCookie_AttachesSession(t *testing.T) {
	t.Parallel()
	want := &domain.Session{UserID: 17, ExpiresAt: time.Now().Add(time.Hour)}
	sess, logger, secure, _ := mwTestRig(&stubSessionService{
		validateFn: func(context.Context, string) (*domain.Session, error) { return want, nil },
	}, false)

	var got *domain.Session
	var ok bool
	wrapped := WithSession(sess, logger, secure)(func(_ http.ResponseWriter, r *http.Request) {
		got, ok = SessionFromContext(r.Context())
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "any-token"})
	wrapped(rr, req)

	if !ok {
		t.Fatalf("session not attached to context")
	}
	if got.UserID != want.UserID {
		t.Errorf("UserID = %d, want %d", got.UserID, want.UserID)
	}
}

func TestWithSession_StaleCookie_ClearsAndPassesAnonymous(t *testing.T) {
	t.Parallel()
	cases := []struct {
		err  error
		name string
	}{
		{name: "expired", err: domain.ErrSessionExpired},
		{name: "not found", err: domain.ErrSessionNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sess, logger, secure, _ := mwTestRig(&stubSessionService{
				validateFn: func(context.Context, string) (*domain.Session, error) { return nil, tc.err },
			}, false)

			called := false
			var ctxHadSession bool
			wrapped := WithSession(sess, logger, secure)(func(_ http.ResponseWriter, r *http.Request) {
				called = true
				_, ctxHadSession = SessionFromContext(r.Context())
			})

			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "stale"})
			wrapped(rr, req)

			if !called {
				t.Errorf("downstream not called after stale cookie cleared")
			}
			if ctxHadSession {
				t.Errorf("context should not contain a session after stale cookie")
			}
			cookie := findSetCookie(rr, sessionCookieName)
			if cookie == nil || cookie.MaxAge >= 0 {
				t.Errorf("stale cookie not cleared: %+v", cookie)
			}
		})
	}
}

func findSetCookie(rr *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, raw := range rr.Result().Cookies() {
		if raw.Name == name {
			return raw
		}
	}
	return nil
}
