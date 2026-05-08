package transport

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hayakawakaki/go-racp/internal/auth/app"
	"github.com/hayakawakaki/go-racp/internal/auth/domain"
)

func newTestHandler(auth *stubAuthService, sess *stubSessionService) *Handler {
	return &Handler{
		svc:     auth,
		sessSvc: sess,
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		secure:  false,
	}
}

func postForm(target string, values map[string]string) *http.Request {
	form := strings.NewReader(encodeForm(values))
	req := httptest.NewRequest(http.MethodPost, target, form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func encodeForm(v map[string]string) string {
	parts := make([]string, 0, len(v))
	for k, val := range v {
		parts = append(parts, k+"="+val)
	}
	return strings.Join(parts, "&")
}

// --- showRegister / showLogin ---

func TestShowRegister_Renders(t *testing.T) {
	t.Parallel()
	h := newTestHandler(&stubAuthService{}, &stubSessionService{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/register", http.NoBody)
	h.showRegister(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `action="/register"`) {
		t.Errorf("body missing register form action")
	}
}

func TestShowLogin_Renders(t *testing.T) {
	t.Parallel()
	h := newTestHandler(&stubAuthService{}, &stubSessionService{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/login", http.NoBody)
	h.showLogin(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `action="/login"`) {
		t.Errorf("body missing login form action")
	}
}

// --- doRegister ---

func TestDoRegister_Happy(t *testing.T) {
	t.Parallel()
	auth := &stubAuthService{}
	h := newTestHandler(auth, &stubSessionService{})

	rr := httptest.NewRecorder()
	req := postForm("/register", map[string]string{
		"username": "alice", "email": "a@x", "password": "pw", "gender": "F",
	})
	h.doRegister(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", rr.Code)
	}
	if rr.Header().Get("Location") != "/login" {
		t.Errorf("Location = %q", rr.Header().Get("Location"))
	}
}

func TestDoRegister_HTMX_Happy(t *testing.T) {
	t.Parallel()
	h := newTestHandler(&stubAuthService{}, &stubSessionService{})

	rr := httptest.NewRecorder()
	req := postForm("/register", map[string]string{"username": "alice", "email": "a@x", "password": "pw"})
	req.Header.Set("HX-Request", "true")
	h.doRegister(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rr.Code)
	}
	if rr.Header().Get("HX-Redirect") != "/login" {
		t.Errorf("HX-Redirect = %q", rr.Header().Get("HX-Redirect"))
	}
}

func TestDoRegister_Errors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		err        error
		wantInBody string
	}{
		{name: "username conflict", err: domain.ErrUsernameConflict, wantInBody: "Username already taken"},
		{name: "email conflict", err: domain.ErrEmailConflict, wantInBody: "Email already in use"},
		{name: "generic", err: errors.New("db down"), wantInBody: "Something went wrong"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			auth := &stubAuthService{
				createFn: func(context.Context, app.CreateCommand) (*app.GetDTO, error) { return nil, tc.err },
			}
			h := newTestHandler(auth, &stubSessionService{})

			rr := httptest.NewRecorder()
			req := postForm("/register", map[string]string{"username": "x", "email": "x@x", "password": "p"})
			h.doRegister(rr, req)

			if !strings.Contains(rr.Body.String(), tc.wantInBody) {
				t.Errorf("body missing %q. got: %s", tc.wantInBody, rr.Body.String())
			}
		})
	}
}

// --- doLogin ---

func TestDoLogin_Happy_SetsCookie(t *testing.T) {
	t.Parallel()
	auth := &stubAuthService{
		authNFn: func(context.Context, app.LoginCommand) (*app.GetDTO, error) {
			return &app.GetDTO{ID: 99, Username: "alice"}, nil
		},
	}
	sess := &stubSessionService{
		createFn: func(_ context.Context, userID int) (string, *domain.Session, error) {
			return "issued-token", &domain.Session{UserID: userID}, nil
		},
	}
	h := newTestHandler(auth, sess)

	rr := httptest.NewRecorder()
	req := postForm("/login", map[string]string{"username": "alice", "password": "pw"})
	h.doLogin(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", rr.Code)
	}
	if rr.Header().Get("Location") != "/" {
		t.Errorf("Location = %q", rr.Header().Get("Location"))
	}
	if len(sess.createCalls) != 1 || sess.createCalls[0] != 99 {
		t.Errorf("createCalls = %v, want [99]", sess.createCalls)
	}
	cookie := findSetCookie(rr, sessionCookieName)
	if cookie == nil {
		t.Fatalf("Set-Cookie %s missing", sessionCookieName)
	}
	if cookie.Value != "issued-token" {
		t.Errorf("cookie.Value = %q, want issued-token", cookie.Value)
	}
	if !cookie.HttpOnly {
		t.Errorf("cookie not HttpOnly")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax", cookie.SameSite)
	}
	if cookie.MaxAge != int(app.SessionTTL.Seconds()) {
		t.Errorf("MaxAge = %d, want %d", cookie.MaxAge, int(app.SessionTTL.Seconds()))
	}
}

func TestDoLogin_InvalidCredentials(t *testing.T) {
	t.Parallel()
	auth := &stubAuthService{
		authNFn: func(context.Context, app.LoginCommand) (*app.GetDTO, error) {
			return nil, domain.ErrInvalidCredentials
		},
	}
	h := newTestHandler(auth, &stubSessionService{})

	rr := httptest.NewRecorder()
	req := postForm("/login", map[string]string{"username": "a", "password": "wrong"})
	h.doLogin(rr, req)

	if !strings.Contains(rr.Body.String(), "Invalid username or password") {
		t.Errorf("body missing invalid-creds message: %s", rr.Body.String())
	}
}

func TestDoLogin_SessionCreateFails(t *testing.T) {
	t.Parallel()
	auth := &stubAuthService{}
	sess := &stubSessionService{
		createFn: func(context.Context, int) (string, *domain.Session, error) { return "", nil, errors.New("boom") },
	}
	h := newTestHandler(auth, sess)

	rr := httptest.NewRecorder()
	req := postForm("/login", map[string]string{"username": "a", "password": "p"})
	h.doLogin(rr, req)

	if !strings.Contains(rr.Body.String(), "Something went wrong") {
		t.Errorf("body missing generic error: %s", rr.Body.String())
	}
	if findSetCookie(rr, sessionCookieName) != nil {
		t.Errorf("cookie should not be set on session create failure")
	}
}

// --- doLogout ---

func TestDoLogout_WithCookie(t *testing.T) {
	t.Parallel()
	sess := &stubSessionService{}
	h := newTestHandler(&stubAuthService{}, sess)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/logout", http.NoBody)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "to-destroy"})
	h.doLogout(rr, req)

	if len(sess.destroyCalls) != 1 || sess.destroyCalls[0] != "to-destroy" {
		t.Errorf("destroyCalls = %v, want [to-destroy]", sess.destroyCalls)
	}
	cookie := findSetCookie(rr, sessionCookieName)
	if cookie == nil || cookie.MaxAge >= 0 {
		t.Errorf("expected cookie cleared, got %+v", cookie)
	}
}

func TestDoLogout_WithoutCookie(t *testing.T) {
	t.Parallel()
	sess := &stubSessionService{}
	h := newTestHandler(&stubAuthService{}, sess)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/logout", http.NoBody)
	h.doLogout(rr, req)

	if len(sess.destroyCalls) != 0 {
		t.Errorf("destroyCalls = %v, want none", sess.destroyCalls)
	}
}

func TestDoLogout_HTMX(t *testing.T) {
	t.Parallel()
	h := newTestHandler(&stubAuthService{}, &stubSessionService{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/logout", http.NoBody)
	req.Header.Set("HX-Request", "true")
	h.doLogout(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rr.Code)
	}
	if rr.Header().Get("HX-Redirect") != "/login" {
		t.Errorf("HX-Redirect = %q", rr.Header().Get("HX-Redirect"))
	}
}
