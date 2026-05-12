package transport

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	accountapp "github.com/hayakawakaki/go-racp/internal/account/app"
	authdomain "github.com/hayakawakaki/go-racp/internal/auth/domain"
)

func TestShowChangeEmail_RendersForm(t *testing.T) {
	t.Parallel()
	h := newTestHandler(&stubAccountService{}, &stubSessionService{}, nil)

	rr := httptest.NewRecorder()
	req := reqWithSession(http.MethodGet, "/account/email", 1, http.NoBody)
	h.showChangeEmail(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `action="/account/email"`) {
		t.Errorf("body missing form action: %s", rr.Body.String())
	}
}

func TestDoChangeEmail_NoSession_RedirectsToLogin(t *testing.T) {
	t.Parallel()
	h := newTestHandler(&stubAccountService{}, &stubSessionService{}, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/account/email", http.NoBody)
	h.doChangeEmail(rr, req)

	if rr.Header().Get("Location") != "/login" {
		t.Errorf("Location = %q, want /login", rr.Header().Get("Location"))
	}
}

func TestDoChangeEmail_HappyPath_RedirectsWithNotice(t *testing.T) {
	t.Parallel()
	svc := &stubAccountService{}
	h := newTestHandler(svc, &stubSessionService{}, nil)

	rr := httptest.NewRecorder()
	req := postWithSession("/account/email", 1, map[string]string{
		"current_password": "Curr1234!",
		"new_email":        "new@example.com",
	})
	h.doChangeEmail(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", rr.Code)
	}
	if rr.Header().Get("Location") != "/account?notice=email_change_sent" {
		t.Errorf("Location = %q, want /account?notice=email_change_sent", rr.Header().Get("Location"))
	}
	if len(svc.requestEmailCalls) != 1 || svc.requestEmailCalls[0].NewEmail != "new@example.com" {
		t.Errorf("requestEmailCalls = %+v", svc.requestEmailCalls)
	}
}

func TestDoChangeEmail_CooldownErrors_RedirectWithMatchingNotice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err        error
		name       string
		wantNotice string
	}{
		{name: "request cooldown", err: accountapp.ErrEmailChangeCooldown, wantNotice: "email_change_cooldown"},
		{name: "change cooldown", err: authdomain.ErrEmailRecentlyChanged, wantNotice: "email_change_locked"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := &stubAccountService{
				requestEmailChangeFn: func(context.Context, int, string, string) error { return tt.err },
			}
			h := newTestHandler(svc, &stubSessionService{}, nil)

			rr := httptest.NewRecorder()
			req := postWithSession("/account/email", 1, map[string]string{
				"current_password": "Curr1234!",
				"new_email":        "new@example.com",
			})
			h.doChangeEmail(rr, req)

			want := "/account?notice=" + tt.wantNotice
			if rr.Header().Get("Location") != want {
				t.Errorf("Location = %q, want %q", rr.Header().Get("Location"), want)
			}
		})
	}
}

func TestDoChangeEmail_ValidationError_RendersFormWithError(t *testing.T) {
	t.Parallel()
	svc := &stubAccountService{
		requestEmailChangeFn: func(context.Context, int, string, string) error {
			return &authdomain.ValidationError{Fields: authdomain.FieldErrors{"new_email": "already in use"}}
		},
	}
	h := newTestHandler(svc, &stubSessionService{}, nil)

	rr := httptest.NewRecorder()
	req := postWithSession("/account/email", 1, map[string]string{
		"current_password": "Curr1234!",
		"new_email":        "taken@example.com",
	})
	h.doChangeEmail(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "already in use") {
		t.Errorf("body missing error: %s", body)
	}
	if !strings.Contains(body, `value="taken@example.com"`) {
		t.Errorf("new email should repopulate; body: %s", body)
	}
}

func TestDoChangeEmail_GenericError_500(t *testing.T) {
	t.Parallel()
	svc := &stubAccountService{
		requestEmailChangeFn: func(context.Context, int, string, string) error {
			return errors.New("db unreachable")
		},
	}
	h := newTestHandler(svc, &stubSessionService{}, nil)

	rr := httptest.NewRecorder()
	req := postWithSession("/account/email", 1, map[string]string{
		"current_password": "Curr1234!",
		"new_email":        "new@example.com",
	})
	h.doChangeEmail(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
}

func TestDoChangeEmail_HTMX_UsesHXRedirect(t *testing.T) {
	t.Parallel()
	h := newTestHandler(&stubAccountService{}, &stubSessionService{}, nil)

	rr := httptest.NewRecorder()
	req := postWithSession("/account/email", 1, map[string]string{
		"current_password": "Curr1234!",
		"new_email":        "new@example.com",
	})
	req.Header.Set("HX-Request", "true")
	h.doChangeEmail(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rr.Code)
	}
	if rr.Header().Get("HX-Redirect") != "/account?notice=email_change_sent" {
		t.Errorf("HX-Redirect = %q, want /account?notice=email_change_sent", rr.Header().Get("HX-Redirect"))
	}
}
