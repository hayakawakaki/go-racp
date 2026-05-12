package transport

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	authdomain "github.com/hayakawakaki/go-racp/internal/account/domain"
)

func TestShowChangePassword_RendersForm(t *testing.T) {
	t.Parallel()
	h := newTestHandler(&stubAccountService{}, &stubSessionService{}, nil)

	rr := httptest.NewRecorder()
	req := reqWithSession(http.MethodGet, "/account/password", 1, http.NoBody)
	h.showChangePassword(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `action="/account/password"`) {
		t.Errorf("body missing form action: %s", rr.Body.String())
	}
}

func TestDoChangePassword_NoSession_RedirectsToLogin(t *testing.T) {
	t.Parallel()
	h := newTestHandler(&stubAccountService{}, &stubSessionService{}, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/account/password", http.NoBody)
	h.doChangePassword(rr, req)

	if rr.Header().Get("Location") != "/login" {
		t.Errorf("Location = %q, want /login", rr.Header().Get("Location"))
	}
}

func TestDoChangePassword_NoCookie_RedirectsToLogin(t *testing.T) {
	t.Parallel()
	h := newTestHandler(&stubAccountService{}, &stubSessionService{}, nil)

	rr := httptest.NewRecorder()
	req := postWithSession("/account/password", 1, map[string]string{
		"current_password":     "Curr1234!",
		"new_password":         "NewPass1!",
		"new_password_confirm": "NewPass1!",
	})
	h.doChangePassword(rr, req)

	if rr.Header().Get("Location") != "/login" {
		t.Errorf("expected redirect to /login when cookie missing; got %q", rr.Header().Get("Location"))
	}
}

func TestDoChangePassword_HappyPath_RedirectsWithNotice(t *testing.T) {
	t.Parallel()
	svc := &stubAccountService{}
	h := newTestHandler(svc, &stubSessionService{}, nil)

	rr := httptest.NewRecorder()
	req := postWithSession("/account/password", 7, map[string]string{
		"current_password":     "Curr1234!",
		"new_password":         "NewPass1!",
		"new_password_confirm": "NewPass1!",
	})
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "current-token"})
	h.doChangePassword(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", rr.Code)
	}
	if rr.Header().Get("Location") != "/account?notice=password_changed" {
		t.Errorf("Location = %q, want /account?notice=password_changed", rr.Header().Get("Location"))
	}
	if len(svc.updatePasswordCalls) != 1 {
		t.Fatalf("UpdatePassword calls = %d, want 1", len(svc.updatePasswordCalls))
	}
	got := svc.updatePasswordCalls[0]
	if got.UserID != 7 || got.CurrentRawToken != "current-token" || got.CurrentPassword != "Curr1234!" || got.NewPassword != "NewPass1!" {
		t.Errorf("call = %+v", got)
	}
}

func TestDoChangePassword_HTMX_UsesHXRedirect(t *testing.T) {
	t.Parallel()
	h := newTestHandler(&stubAccountService{}, &stubSessionService{}, nil)

	rr := httptest.NewRecorder()
	req := postWithSession("/account/password", 1, map[string]string{
		"current_password":     "Curr1234!",
		"new_password":         "NewPass1!",
		"new_password_confirm": "NewPass1!",
	})
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "tok"})
	req.Header.Set("HX-Request", "true")
	h.doChangePassword(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rr.Code)
	}
	if rr.Header().Get("HX-Redirect") != "/account?notice=password_changed" {
		t.Errorf("HX-Redirect = %q, want /account?notice=password_changed", rr.Header().Get("HX-Redirect"))
	}
}

func TestDoChangePassword_ValidationError_RendersFieldError(t *testing.T) {
	t.Parallel()
	svc := &stubAccountService{
		updatePasswordFn: func(context.Context, int, string, string, string, string) error {
			return &authdomain.ValidationError{Fields: authdomain.FieldErrors{
				"new_password": "password must contain a digit",
			}}
		},
	}
	h := newTestHandler(svc, &stubSessionService{}, nil)

	rr := httptest.NewRecorder()
	req := postWithSession("/account/password", 1, map[string]string{
		"current_password":     "Curr1234!",
		"new_password":         "WeakPass!",
		"new_password_confirm": "WeakPass!",
	})
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "tok"})
	h.doChangePassword(rr, req)

	if !strings.Contains(rr.Body.String(), "password must contain a digit") {
		t.Errorf("body missing field error: %s", rr.Body.String())
	}
}

func TestDoChangePassword_RecentlyChanged_RendersFriendlyMessage(t *testing.T) {
	t.Parallel()
	svc := &stubAccountService{
		updatePasswordFn: func(context.Context, int, string, string, string, string) error {
			return authdomain.ErrPasswordRecentlyChanged
		},
	}
	h := newTestHandler(svc, &stubSessionService{}, nil)

	rr := httptest.NewRecorder()
	req := postWithSession("/account/password", 1, map[string]string{
		"current_password":     "Curr1234!",
		"new_password":         "NewPass1!",
		"new_password_confirm": "NewPass1!",
	})
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "tok"})
	h.doChangePassword(rr, req)

	if !strings.Contains(rr.Body.String(), "Password was changed recently") {
		t.Errorf("body missing friendly recent-change message: %s", rr.Body.String())
	}
}

func TestDoChangePassword_GenericError_500(t *testing.T) {
	t.Parallel()
	svc := &stubAccountService{
		updatePasswordFn: func(context.Context, int, string, string, string, string) error {
			return errors.New("db unreachable")
		},
	}
	h := newTestHandler(svc, &stubSessionService{}, nil)

	rr := httptest.NewRecorder()
	req := postWithSession("/account/password", 1, map[string]string{
		"current_password":     "Curr1234!",
		"new_password":         "NewPass1!",
		"new_password_confirm": "NewPass1!",
	})
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "tok"})
	h.doChangePassword(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
}
