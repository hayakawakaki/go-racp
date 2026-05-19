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

	actiontokendomain "github.com/hayakawakaki/go-racp/internal/actiontoken/domain"
	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
)

func newResetHandler(reset *stubAccountService) *Handler {
	return &Handler{
		svc:    reset,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func TestShowResetPassword_NoToken_ReturnsNotFound(t *testing.T) {
	t.Parallel()
	h := newResetHandler(&stubAccountService{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/reset-password", http.NoBody)
	h.showResetPassword(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestShowResetPassword_PeekExpired_RendersExpired(t *testing.T) {
	t.Parallel()
	h := newResetHandler(&stubAccountService{
		peekFn: func(context.Context, actiontokendomain.Action, string) (*actiontokendomain.ActionToken, error) {
			return nil, actiontokendomain.ErrTokenExpired
		},
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/reset-password?token=abc", http.NoBody)
	h.showResetPassword(rr, req)

	if !strings.Contains(rr.Body.String(), "Link expired") {
		t.Errorf("body missing %q: %s", "Link expired", rr.Body.String())
	}
}

func TestShowResetPassword_PeekInvalidOrAlreadyUsed_ReturnsNotFound(t *testing.T) {
	t.Parallel()

	tests := []struct {
		peekErr error
		name    string
	}{
		{name: "already used", peekErr: actiontokendomain.ErrTokenAlreadyUsed},
		{name: "invalid", peekErr: actiontokendomain.ErrTokenInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newResetHandler(&stubAccountService{
				peekFn: func(context.Context, actiontokendomain.Action, string) (*actiontokendomain.ActionToken, error) {
					return nil, tt.peekErr
				},
			})

			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/reset-password?token=abc", http.NoBody)
			h.showResetPassword(rr, req)

			if rr.Code != http.StatusNotFound {
				t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
			}
		})
	}
}

func TestShowResetPassword_PeekGenericError_ReturnsNotFound(t *testing.T) {
	t.Parallel()
	h := newResetHandler(&stubAccountService{
		peekFn: func(context.Context, actiontokendomain.Action, string) (*actiontokendomain.ActionToken, error) {
			return nil, errors.New("db unreachable")
		},
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/reset-password?token=abc", http.NoBody)
	h.showResetPassword(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestShowResetPassword_ValidToken_RendersFormWithHiddenToken(t *testing.T) {
	t.Parallel()
	h := newResetHandler(&stubAccountService{
		peekFn: func(context.Context, actiontokendomain.Action, string) (*actiontokendomain.ActionToken, error) {
			return &actiontokendomain.ActionToken{AccountID: 1}, nil
		},
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/reset-password?token=abc", http.NoBody)
	h.showResetPassword(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, `name="token"`) || !strings.Contains(body, `value="abc"`) {
		t.Errorf("body missing hidden token field carrying %q: %s", "abc", body)
	}
}

func TestShowResetPassword_SetsNoReferrerHeader(t *testing.T) {
	t.Parallel()
	h := newResetHandler(&stubAccountService{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/reset-password", http.NoBody)
	h.showResetPassword(rr, req)

	if rr.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Errorf("Referrer-Policy = %q, want no-referrer", rr.Header().Get("Referrer-Policy"))
	}
}

func TestDoResetPassword_PasswordMismatch_RendersFieldError(t *testing.T) {
	t.Parallel()
	h := newResetHandler(&stubAccountService{})

	rr := httptest.NewRecorder()
	req := postForm("/reset-password", map[string]string{
		"token":            "abc",
		"password":         "NewPass1!",
		"password_confirm": "OtherPass1!",
	})
	h.doResetPassword(rr, req)

	if !strings.Contains(rr.Body.String(), "passwords do not match") {
		t.Errorf("body missing mismatch error: %s", rr.Body.String())
	}
}

func TestDoResetPassword_ValidationError_RendersFieldError(t *testing.T) {
	t.Parallel()
	h := newResetHandler(&stubAccountService{
		consumeResetFn: func(context.Context, string, string) error {
			return &domain.ValidationError{Fields: domain.FieldErrors{"password": "password must contain a digit"}}
		},
	})

	rr := httptest.NewRecorder()
	req := postForm("/reset-password", map[string]string{
		"token":            "abc",
		"password":         "NewPass!",
		"password_confirm": "NewPass!",
	})
	h.doResetPassword(rr, req)

	if !strings.Contains(rr.Body.String(), "password must contain a digit") {
		t.Errorf("body missing validation message: %s", rr.Body.String())
	}
}

func TestDoResetPassword_TokenErrors_RendersMatchingResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		consumeErr error
		name       string
		wantText   string
	}{
		{name: "expired", consumeErr: actiontokendomain.ErrTokenExpired, wantText: "Link expired"},
		{name: "already used", consumeErr: actiontokendomain.ErrTokenAlreadyUsed, wantText: "Link already used"},
		{name: "invalid", consumeErr: actiontokendomain.ErrTokenInvalid, wantText: "Invalid link"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newResetHandler(&stubAccountService{
				consumeResetFn: func(context.Context, string, string) error { return tt.consumeErr },
			})

			rr := httptest.NewRecorder()
			req := postForm("/reset-password", map[string]string{
				"token":            "abc",
				"password":         "NewPass1!",
				"password_confirm": "NewPass1!",
			})
			h.doResetPassword(rr, req)

			if !strings.Contains(rr.Body.String(), tt.wantText) {
				t.Errorf("body missing %q: %s", tt.wantText, rr.Body.String())
			}
		})
	}
}

func TestDoResetPassword_AccountRestricted_RendersRestrictedResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		consumeErr error
		name       string
	}{
		{name: "perma banned", consumeErr: domain.ErrAccountPermaBanned},
		{name: "temp banned", consumeErr: domain.ErrAccountTempBanned},
		{name: "deleted", consumeErr: domain.ErrAccountDeleted},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newResetHandler(&stubAccountService{
				consumeResetFn: func(context.Context, string, string) error { return tt.consumeErr },
			})

			rr := httptest.NewRecorder()
			req := postForm("/reset-password", map[string]string{
				"token":            "abc",
				"password":         "NewPass1!",
				"password_confirm": "NewPass1!",
			})
			h.doResetPassword(rr, req)

			body := rr.Body.String()
			if !strings.Contains(body, "Account restricted") {
				t.Errorf("body missing restricted heading: %s", body)
			}
			if !strings.Contains(body, "cannot have its password reset") {
				t.Errorf("body missing restricted explanation: %s", body)
			}
		})
	}
}

func TestDoResetPassword_Success_RendersSuccess(t *testing.T) {
	t.Parallel()
	h := newResetHandler(&stubAccountService{
		consumeResetFn: func(context.Context, string, string) error { return nil },
	})

	rr := httptest.NewRecorder()
	req := postForm("/reset-password", map[string]string{
		"token":            "abc",
		"password":         "NewPass1!",
		"password_confirm": "NewPass1!",
	})
	h.doResetPassword(rr, req)

	if !strings.Contains(rr.Body.String(), "Password reset") {
		t.Errorf("body missing success: %s", rr.Body.String())
	}
}
