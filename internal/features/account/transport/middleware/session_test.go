package middleware

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
)

func mwTestRig(sess *stubSessionService, secure bool) (*stubSessionService, *slog.Logger, bool, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(buf, nil))
	return sess, logger, secure, buf
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
	if findSetCookie(rr, SessionCookieName) != nil {
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
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "any-token"})
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
			req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "stale"})
			wrapped(rr, req)

			if !called {
				t.Errorf("downstream not called after stale cookie cleared")
			}
			if ctxHadSession {
				t.Errorf("context should not contain a session after stale cookie")
			}
			cookie := findSetCookie(rr, SessionCookieName)
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
