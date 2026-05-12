package transport

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hayakawakaki/go-racp/internal/account/domain"
)

func newForgotHandler(reset *stubAccountService, logBuffer io.Writer) *Handler {
	if logBuffer == nil {
		logBuffer = io.Discard
	}
	return &Handler{
		svc:    reset,
		logger: slog.New(slog.NewTextHandler(logBuffer, nil)),
	}
}

func TestShowForgotPassword_Renders(t *testing.T) {
	t.Parallel()
	h := newForgotHandler(&stubAccountService{}, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/forgot-password", http.NoBody)
	h.showForgotPassword(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `action="/forgot-password"`) {
		t.Errorf("body missing forgot-password form action: %s", rr.Body.String())
	}
}

func TestDoForgotPassword_Success_RendersSubmittedNotice(t *testing.T) {
	t.Parallel()
	calls := 0
	reset := &stubAccountService{
		requestResetFn: func(_ context.Context, email string) error {
			calls++
			if email != "user@example.com" {
				t.Errorf("email = %q, want user@example.com", email)
			}
			return nil
		},
	}
	h := newForgotHandler(reset, nil)

	rr := httptest.NewRecorder()
	req := postForm("/forgot-password", map[string]string{"email": "user@example.com"})
	h.doForgotPassword(rr, req)

	if calls != 1 {
		t.Errorf("RequestPasswordReset calls = %d, want 1", calls)
	}
	if !strings.Contains(rr.Body.String(), "Check your inbox") {
		t.Errorf("body missing submitted notice: %s", rr.Body.String())
	}
}

func TestDoForgotPassword_ValidationError_RendersFieldError(t *testing.T) {
	t.Parallel()
	reset := &stubAccountService{
		requestResetFn: func(context.Context, string) error {
			return &domain.ValidationError{Fields: domain.FieldErrors{"email": "email is not a valid address"}}
		},
	}
	h := newForgotHandler(reset, nil)

	rr := httptest.NewRecorder()
	req := postForm("/forgot-password", map[string]string{"email": "bad"})
	h.doForgotPassword(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "email is not a valid address") {
		t.Errorf("body missing email error: %s", body)
	}
	if !strings.Contains(body, `value="bad"`) {
		t.Errorf("email value should repopulate; body: %s", body)
	}
}

func TestDoForgotPassword_GenericError_LogsAndShowsGenericMessage(t *testing.T) {
	t.Parallel()
	logBuffer := &bytes.Buffer{}
	reset := &stubAccountService{
		requestResetFn: func(context.Context, string) error { return errors.New("db unreachable") },
	}
	h := newForgotHandler(reset, logBuffer)

	rr := httptest.NewRecorder()
	req := postForm("/forgot-password", map[string]string{"email": "user@example.com"})
	h.doForgotPassword(rr, req)

	if !strings.Contains(rr.Body.String(), "Something went wrong") {
		t.Errorf("body missing generic message: %s", rr.Body.String())
	}
	if !strings.Contains(logBuffer.String(), "forgot_password") {
		t.Errorf("expected forgot_password in log, got %q", logBuffer.String())
	}
}

func TestDoForgotPassword_HTMX_RendersForm(t *testing.T) {
	t.Parallel()
	h := newForgotHandler(&stubAccountService{}, nil)

	rr := httptest.NewRecorder()
	req := postForm("/forgot-password", map[string]string{"email": "user@example.com"})
	req.Header.Set("HX-Request", "true")
	h.doForgotPassword(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
}
