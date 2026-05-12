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

	"github.com/hayakawakaki/go-racp/internal/account/domain"
	"github.com/hayakawakaki/go-racp/internal/actiontoken"
)

func newResetHandler(reset *stubAccountService) *Handler {
	return &Handler{
		svc:    reset,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func TestShowResetPassword_NoToken_RendersInvalid(t *testing.T) {
	t.Parallel()
	h := newResetHandler(&stubAccountService{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/reset-password", http.NoBody)
	h.showResetPassword(rr, req)

	if !strings.Contains(rr.Body.String(), "Invalid link") {
		t.Errorf("body should render invalid result: %s", rr.Body.String())
	}
}

func TestShowResetPassword_PeekKnownErrors_RendersMatchingResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		peekErr  error
		name     string
		wantText string
	}{
		{name: "expired", peekErr: actiontoken.ErrTokenExpired, wantText: "Link expired"},
		{name: "already used", peekErr: actiontoken.ErrTokenAlreadyUsed, wantText: "Link already used"},
		{name: "invalid", peekErr: actiontoken.ErrTokenInvalid, wantText: "Invalid link"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newResetHandler(&stubAccountService{
				peekResetFn: func(context.Context, string) (*actiontoken.ActionToken, error) {
					return nil, tt.peekErr
				},
			})

			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/reset-password?token=abc", http.NoBody)
			h.showResetPassword(rr, req)

			if !strings.Contains(rr.Body.String(), tt.wantText) {
				t.Errorf("body missing %q: %s", tt.wantText, rr.Body.String())
			}
		})
	}
}

func TestShowResetPassword_PeekGenericError_RendersInvalid(t *testing.T) {
	t.Parallel()
	h := newResetHandler(&stubAccountService{
		peekResetFn: func(context.Context, string) (*actiontoken.ActionToken, error) {
			return nil, errors.New("db unreachable")
		},
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/reset-password?token=abc", http.NoBody)
	h.showResetPassword(rr, req)

	if !strings.Contains(rr.Body.String(), "Invalid link") {
		t.Errorf("body should render invalid for opaque error: %s", rr.Body.String())
	}
}

func TestShowResetPassword_ValidToken_RendersFormWithHiddenToken(t *testing.T) {
	t.Parallel()
	h := newResetHandler(&stubAccountService{
		peekResetFn: func(context.Context, string) (*actiontoken.ActionToken, error) {
			return &actiontoken.ActionToken{AccountID: 1}, nil
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
		{name: "expired", consumeErr: actiontoken.ErrTokenExpired, wantText: "Link expired"},
		{name: "already used", consumeErr: actiontoken.ErrTokenAlreadyUsed, wantText: "Link already used"},
		{name: "invalid", consumeErr: actiontoken.ErrTokenInvalid, wantText: "Invalid link"},
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
