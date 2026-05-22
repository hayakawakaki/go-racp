package self

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	app "github.com/hayakawakaki/go-racp/internal/features/account/app/self"
	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
	"github.com/hayakawakaki/go-racp/internal/features/account/transport/middleware"
)

func findSetCookie(rr *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, raw := range rr.Result().Cookies() {
		if raw.Name == name {
			return raw
		}
	}
	return nil
}

func newAuthHandler(auth *stubAccountService, sess *stubSessionService) *Handler {
	return &Handler{
		svc:     auth,
		sessSvc: sess,
		theme:   stubTheme{},
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		secure:  false,
	}
}

// --- showRegister / showLogin ---

func TestShowRegister_Renders(t *testing.T) {
	t.Parallel()
	h := newAuthHandler(&stubAccountService{}, &stubSessionService{})

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
	h := newAuthHandler(&stubAccountService{}, &stubSessionService{})

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
	auth := &stubAccountService{}
	h := newAuthHandler(auth, &stubSessionService{})

	rr := httptest.NewRecorder()
	req := postForm("/register", map[string]string{
		"username":         "testuser",
		"email":            "test@x",
		"password":         "Test1234!",
		"password_confirm": "Test1234!",
		"gender":           "F",
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
	h := newAuthHandler(&stubAccountService{}, &stubSessionService{})

	rr := httptest.NewRecorder()
	req := postForm("/register", map[string]string{
		"username":         "testuser",
		"email":            "test@x",
		"password":         "Test1234!",
		"password_confirm": "Test1234!",
		"gender":           "F",
	})
	req.Header.Set("HX-Request", "true")
	h.doRegister(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rr.Code)
	}
	if rr.Header().Get("HX-Redirect") != "/login" {
		t.Errorf("HX-Redirect = %q", rr.Header().Get("HX-Redirect"))
	}
}

func TestDoRegister_FieldValidationErrors(t *testing.T) {
	t.Parallel()
	auth := &stubAccountService{
		createFn: func(context.Context, app.CreateCommand) (*app.GetDTO, error) {
			return nil, &domain.ValidationError{Fields: domain.FieldErrors{
				"username":         "username must be at least 6 characters",
				"password_confirm": "passwords do not match",
			}}
		},
	}
	h := newAuthHandler(auth, &stubSessionService{})

	rr := httptest.NewRecorder()
	req := postForm("/register", map[string]string{
		"username": "abc", "email": "x@x", "password": "Test1234!", "password_confirm": "Wrong1234!",
	})
	h.doRegister(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "username must be at least 6 characters") {
		t.Errorf("body missing username error: %s", body)
	}
	if !strings.Contains(body, "passwords do not match") {
		t.Errorf("body missing password_confirm error: %s", body)
	}
	if !strings.Contains(body, `value="abc"`) {
		t.Errorf("username should repopulate")
	}
	if strings.Contains(body, `value="Test1234!"`) {
		t.Errorf("password should not repopulate")
	}
}

func TestDoRegister_PassesBirthdate(t *testing.T) {
	t.Parallel()
	var captured app.CreateCommand
	auth := &stubAccountService{
		createFn: func(_ context.Context, cmd app.CreateCommand) (*app.GetDTO, error) {
			captured = cmd
			return &app.GetDTO{ID: 1, Username: cmd.Username, Email: cmd.Email}, nil
		},
	}
	h := newAuthHandler(auth, &stubSessionService{})

	rr := httptest.NewRecorder()
	req := postForm("/register", map[string]string{
		"username":         "testuser",
		"email":            "test@x",
		"password":         "Test1234!",
		"password_confirm": "Test1234!",
		"gender":           "F",
		"birthdate":        "2000-01-15",
	})
	h.doRegister(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rr.Code)
	}
	if captured.Birthdate != "2000-01-15" {
		t.Errorf("Birthdate = %q; want 2000-01-15", captured.Birthdate)
	}
}

func TestDoRegister_BirthdateErrorIsRendered(t *testing.T) {
	t.Parallel()
	auth := &stubAccountService{
		createFn: func(context.Context, app.CreateCommand) (*app.GetDTO, error) {
			return nil, &domain.ValidationError{Fields: domain.FieldErrors{
				"birthdate": "birthdate is required",
			}}
		},
	}
	h := newAuthHandler(auth, &stubSessionService{})

	rr := httptest.NewRecorder()
	req := postForm("/register", map[string]string{
		"username":         "testuser",
		"email":            "test@x",
		"password":         "Test1234!",
		"password_confirm": "Test1234!",
		"gender":           "F",
		"birthdate":        "",
	})
	h.doRegister(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "birthdate is required") {
		t.Errorf("body missing birthdate error; got: %s", body)
	}
	if !strings.Contains(body, `value="testuser"`) {
		t.Errorf("username not echoed back on error")
	}
}

func TestDoRegister_GenericError(t *testing.T) {
	t.Parallel()
	auth := &stubAccountService{
		createFn: func(context.Context, app.CreateCommand) (*app.GetDTO, error) {
			return nil, errors.New("db down")
		},
	}
	h := newAuthHandler(auth, &stubSessionService{})

	rr := httptest.NewRecorder()
	req := postForm("/register", map[string]string{
		"username": "testuser", "email": "test@x", "password": "Test1234!", "password_confirm": "Test1234!",
	})
	h.doRegister(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Something went wrong") {
		t.Errorf("body missing generic error: %s", rr.Body.String())
	}
}

// --- doLogin ---

func TestDoLogin_Happy_SetsCookie(t *testing.T) {
	t.Parallel()
	auth := &stubAccountService{
		authNFn: func(context.Context, app.LoginCommand) (*app.GetDTO, app.Tier, error) {
			return &app.GetDTO{ID: 99, Username: "testuser"}, app.TierActive, nil
		},
	}
	sess := &stubSessionService{
		createFn: func(_ context.Context, userID int) (string, *domain.Session, error) {
			return "issued-token", &domain.Session{UserID: userID}, nil
		},
	}
	h := newAuthHandler(auth, sess)

	rr := httptest.NewRecorder()
	req := postForm("/login", map[string]string{"username": "testuser", "password": "Test1234!"})
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
	cookie := findSetCookie(rr, middleware.SessionCookieName)
	if cookie == nil {
		t.Fatalf("Set-Cookie %s missing", middleware.SessionCookieName)
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
	if cookie.MaxAge != int(stubSessionTTL.Seconds()) {
		t.Errorf("MaxAge = %d, want %d", cookie.MaxAge, int(stubSessionTTL.Seconds()))
	}
}

func TestDoLogin_InvalidCredentials(t *testing.T) {
	t.Parallel()
	auth := &stubAccountService{
		authNFn: func(context.Context, app.LoginCommand) (*app.GetDTO, app.Tier, error) {
			return nil, app.TierActive, domain.ErrInvalidCredentials
		},
	}
	h := newAuthHandler(auth, &stubSessionService{})

	rr := httptest.NewRecorder()
	req := postForm("/login", map[string]string{"username": "a", "password": "wrong"})
	h.doLogin(rr, req)

	if !strings.Contains(rr.Body.String(), "Invalid username or password") {
		t.Errorf("body missing invalid-creds message: %s", rr.Body.String())
	}
}

func TestDoLogin_AccountLocked_DistinctMessage(t *testing.T) {
	t.Parallel()
	auth := &stubAccountService{
		authNFn: func(context.Context, app.LoginCommand) (*app.GetDTO, app.Tier, error) {
			return nil, app.TierActive, domain.ErrAccountLocked
		},
	}
	h := newAuthHandler(auth, &stubSessionService{})

	rr := httptest.NewRecorder()
	req := postForm("/login", map[string]string{"username": "testuser", "password": "Test1234!"})
	h.doLogin(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "Too many recent attempts") {
		t.Errorf("body missing lockout-specific message: %s", body)
	}
	if strings.Contains(body, "Invalid username or password") {
		t.Errorf("lockout response must not show the invalid-credentials message: %s", body)
	}
	if findSetCookie(rr, middleware.SessionCookieName) != nil {
		t.Errorf("cookie must not be set on lockout")
	}
}

func TestDoLogin_SessionCreateFails(t *testing.T) {
	t.Parallel()
	auth := &stubAccountService{}
	sess := &stubSessionService{
		createFn: func(context.Context, int) (string, *domain.Session, error) { return "", nil, errors.New("boom") },
	}
	h := newAuthHandler(auth, sess)

	rr := httptest.NewRecorder()
	req := postForm("/login", map[string]string{"username": "a", "password": "p"})
	h.doLogin(rr, req)

	if !strings.Contains(rr.Body.String(), "Something went wrong") {
		t.Errorf("body missing generic error: %s", rr.Body.String())
	}
	if findSetCookie(rr, middleware.SessionCookieName) != nil {
		t.Errorf("cookie should not be set on session create failure")
	}
}

// --- doLogout ---

func TestDoLogout_WithCookie(t *testing.T) {
	t.Parallel()
	sess := &stubSessionService{}
	h := newAuthHandler(&stubAccountService{}, sess)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/logout", http.NoBody)
	req.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: "to-destroy"})
	h.doLogout(rr, req)

	if len(sess.destroyCalls) != 1 || sess.destroyCalls[0] != "to-destroy" {
		t.Errorf("destroyCalls = %v, want [to-destroy]", sess.destroyCalls)
	}
	cookie := findSetCookie(rr, middleware.SessionCookieName)
	if cookie == nil || cookie.MaxAge >= 0 {
		t.Errorf("expected cookie cleared, got %+v", cookie)
	}
}

func TestDoLogout_WithoutCookie(t *testing.T) {
	t.Parallel()
	sess := &stubSessionService{}
	h := newAuthHandler(&stubAccountService{}, sess)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/logout", http.NoBody)
	h.doLogout(rr, req)

	if len(sess.destroyCalls) != 0 {
		t.Errorf("destroyCalls = %v, want none", sess.destroyCalls)
	}
}

func TestDoLogout_HTMX(t *testing.T) {
	t.Parallel()
	h := newAuthHandler(&stubAccountService{}, &stubSessionService{})

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
