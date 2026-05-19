package self

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	app "github.com/hayakawakaki/go-racp/internal/features/account/app/self"
)

func TestShowAccount_NoSessionInContext_RedirectsToLogin(t *testing.T) {
	t.Parallel()
	h := newTestHandler(&stubAccountService{}, &stubSessionService{}, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/account", http.NoBody)
	h.showAccount(rr, req)

	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/login" {
		t.Errorf("status = %d, Location = %q; want 303 /login", rr.Code, rr.Header().Get("Location"))
	}
}

func TestShowAccount_RendersAccountDetails(t *testing.T) {
	t.Parallel()
	svc := &stubAccountService{
		getAccountFn: func(context.Context, int) (*app.AccountDTO, error) {
			return &app.AccountDTO{Username: "alice", Email: "alice@example.com", Verified: true}, nil
		},
	}
	h := newTestHandler(svc, &stubSessionService{}, nil)

	rr := httptest.NewRecorder()
	req := reqWithSession(http.MethodGet, "/account", 1, http.NoBody)
	h.showAccount(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "alice") || !strings.Contains(body, "alice@example.com") {
		t.Errorf("body missing account details: %s", body)
	}
}

func TestShowAccount_ServiceError_500(t *testing.T) {
	t.Parallel()
	svc := &stubAccountService{
		getAccountFn: func(context.Context, int) (*app.AccountDTO, error) {
			return nil, errors.New("db unreachable")
		},
	}
	h := newTestHandler(svc, &stubSessionService{}, nil)

	rr := httptest.NewRecorder()
	req := reqWithSession(http.MethodGet, "/account", 1, http.NoBody)
	h.showAccount(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
}

func TestShowAccount_KnownNotice_Rendered(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		notice   string
		wantText string
		want     bool
	}{
		{name: "password_changed", notice: "password_changed", wantText: "Password updated.", want: true},
		{name: "email_change_sent", notice: "email_change_sent", wantText: "sent a confirmation link", want: true},
		{name: "email_change_cooldown", notice: "email_change_cooldown", wantText: "We sent a confirmation link recently", want: true},
		{name: "email_change_locked", notice: "email_change_locked", wantText: "Email was changed recently", want: true},
		{name: "email_changed", notice: "email_changed", wantText: "Email updated.", want: true},
		{name: "unknown notice silently ignored", notice: "garbage", wantText: "Password updated.", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newTestHandler(&stubAccountService{}, &stubSessionService{}, nil)

			rr := httptest.NewRecorder()
			target := "/account?notice=" + tt.notice
			req := reqWithSession(http.MethodGet, target, 1, http.NoBody)
			h.showAccount(rr, req)

			if got := strings.Contains(rr.Body.String(), tt.wantText); got != tt.want {
				t.Errorf("contains %q = %v, want %v; body: %s", tt.wantText, got, tt.want, rr.Body.String())
			}
		})
	}
}
