package transport

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	authdomain "github.com/hayakawakaki/go-racp/internal/account/domain"
	"github.com/hayakawakaki/go-racp/internal/actiontoken"
)

func TestDoVerifyEmailChange_NoToken_RendersInvalid(t *testing.T) {
	t.Parallel()
	h := newTestHandler(&stubAccountService{}, &stubSessionService{}, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/verify-email-change", http.NoBody)
	h.doVerifyEmailChange(rr, req)

	if !strings.Contains(rr.Body.String(), "Invalid link") {
		t.Errorf("body missing invalid result: %s", rr.Body.String())
	}
}

func TestDoVerifyEmailChange_SetsNoReferrerHeader(t *testing.T) {
	t.Parallel()
	h := newTestHandler(&stubAccountService{}, &stubSessionService{}, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/verify-email-change", http.NoBody)
	h.doVerifyEmailChange(rr, req)

	if rr.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Errorf("Referrer-Policy = %q, want no-referrer", rr.Header().Get("Referrer-Policy"))
	}
}

func TestDoVerifyEmailChange_TokenErrors_RenderMatchingResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		consumeErr error
		name       string
		wantText   string
	}{
		{name: "expired", consumeErr: actiontoken.ErrTokenExpired, wantText: "Link expired"},
		{name: "already used", consumeErr: actiontoken.ErrTokenAlreadyUsed, wantText: "Link already used"},
		{name: "invalid", consumeErr: actiontoken.ErrTokenInvalid, wantText: "Invalid link"},
		{name: "email taken", consumeErr: authdomain.ErrEmailTaken, wantText: "Email no longer available"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := &stubAccountService{
				consumeEmailChangeFn: func(context.Context, string) (*authdomain.User, error) {
					return nil, tt.consumeErr
				},
			}
			h := newTestHandler(svc, &stubSessionService{}, nil)

			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/verify-email-change?token=abc", http.NoBody)
			h.doVerifyEmailChange(rr, req)

			if !strings.Contains(rr.Body.String(), tt.wantText) {
				t.Errorf("body missing %q: %s", tt.wantText, rr.Body.String())
			}
		})
	}
}

func TestDoVerifyEmailChange_GenericError_RendersInvalid(t *testing.T) {
	t.Parallel()
	svc := &stubAccountService{
		consumeEmailChangeFn: func(context.Context, string) (*authdomain.User, error) {
			return nil, errors.New("db unreachable")
		},
	}
	h := newTestHandler(svc, &stubSessionService{}, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/verify-email-change?token=abc", http.NoBody)
	h.doVerifyEmailChange(rr, req)

	if !strings.Contains(rr.Body.String(), "Invalid link") {
		t.Errorf("body missing invalid result for opaque error: %s", rr.Body.String())
	}
}

func TestDoVerifyEmailChange_Success_RendersSuccess(t *testing.T) {
	t.Parallel()
	svc := &stubAccountService{
		consumeEmailChangeFn: func(context.Context, string) (*authdomain.User, error) {
			return &authdomain.User{Email: "new@example.com"}, nil
		},
	}
	h := newTestHandler(svc, &stubSessionService{}, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/verify-email-change?token=abc", http.NoBody)
	h.doVerifyEmailChange(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "Email changed") {
		t.Errorf("body missing success: %s", body)
	}
	if !strings.Contains(body, "new@example.com") {
		t.Errorf("body should display new email: %s", body)
	}
}
