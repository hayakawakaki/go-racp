package self

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

	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
	"github.com/hayakawakaki/go-racp/internal/features/account/transport/middleware"
	actiontokendomain "github.com/hayakawakaki/go-racp/internal/platform/actiontoken/domain"
)

func newVerifyHandler(users *stubUserLookup, sess *stubSessionService, verify *stubAccountService, logBuffer io.Writer) *Handler {
	if logBuffer == nil {
		logBuffer = io.Discard
	}
	return &Handler{
		users:   users,
		sessSvc: sess,
		svc:     verify,
		logger:  slog.New(slog.NewTextHandler(logBuffer, nil)),
	}
}

func withSessionCookie(req *http.Request, value string) *http.Request {
	req.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: value})
	return req
}

func TestShowVerifyAccount_NoCookie_RedirectsToLogin(t *testing.T) {
	t.Parallel()
	h := newVerifyHandler(&stubUserLookup{}, &stubSessionService{}, &stubAccountService{}, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/verify-account", http.NoBody)
	h.showVerifyAccount(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusSeeOther)
	}
	if rr.Header().Get("Location") != "/login" {
		t.Errorf("Location = %q, want /login", rr.Header().Get("Location"))
	}
}

func TestShowVerifyAccount_EmptyCookie_RedirectsToLogin(t *testing.T) {
	t.Parallel()
	h := newVerifyHandler(&stubUserLookup{}, &stubSessionService{}, &stubAccountService{}, nil)

	rr := httptest.NewRecorder()
	req := withSessionCookie(httptest.NewRequest(http.MethodGet, "/verify-account", http.NoBody), "")
	h.showVerifyAccount(rr, req)

	if rr.Header().Get("Location") != "/login" {
		t.Errorf("Location = %q, want /login", rr.Header().Get("Location"))
	}
}

func TestShowVerifyAccount_InvalidSession_RedirectsToLogin(t *testing.T) {
	t.Parallel()
	sess := &stubSessionService{
		validateFn: func(context.Context, string) (*domain.Session, error) {
			return nil, domain.ErrSessionExpired
		},
	}
	h := newVerifyHandler(&stubUserLookup{}, sess, &stubAccountService{}, nil)

	rr := httptest.NewRecorder()
	req := withSessionCookie(httptest.NewRequest(http.MethodGet, "/verify-account", http.NoBody), "stale")
	h.showVerifyAccount(rr, req)

	if rr.Header().Get("Location") != "/login" {
		t.Errorf("Location = %q, want /login", rr.Header().Get("Location"))
	}
}

func TestShowVerifyAccount_UserLookupError_RedirectsToLogin(t *testing.T) {
	t.Parallel()
	sess := &stubSessionService{
		validateFn: func(context.Context, string) (*domain.Session, error) {
			return &domain.Session{UserID: 1}, nil
		},
	}
	users := &stubUserLookup{
		getByIDFn: func(context.Context, int) (*domain.User, error) {
			return nil, errors.New("user lookup boom")
		},
	}
	h := newVerifyHandler(users, sess, &stubAccountService{}, nil)

	rr := httptest.NewRecorder()
	req := withSessionCookie(httptest.NewRequest(http.MethodGet, "/verify-account", http.NoBody), "ok")
	h.showVerifyAccount(rr, req)

	if rr.Header().Get("Location") != "/login" {
		t.Errorf("Location = %q, want /login", rr.Header().Get("Location"))
	}
}

func TestShowVerifyAccount_AlreadyVerified_RedirectsToHome(t *testing.T) {
	t.Parallel()
	sess := &stubSessionService{
		validateFn: func(context.Context, string) (*domain.Session, error) {
			return &domain.Session{UserID: 1}, nil
		},
	}
	users := &stubUserLookup{
		getByIDFn: func(_ context.Context, id int) (*domain.User, error) {
			return &domain.User{ID: id, State: 0, Email: "v@x"}, nil
		},
	}
	h := newVerifyHandler(users, sess, &stubAccountService{}, nil)

	rr := httptest.NewRecorder()
	req := withSessionCookie(httptest.NewRequest(http.MethodGet, "/verify-account", http.NoBody), "ok")
	h.showVerifyAccount(rr, req)

	if rr.Header().Get("Location") != "/" {
		t.Errorf("Location = %q, want /", rr.Header().Get("Location"))
	}
}

func TestShowVerifyAccount_Unverified_RendersPageWithEmail(t *testing.T) {
	t.Parallel()
	sess := &stubSessionService{
		validateFn: func(context.Context, string) (*domain.Session, error) {
			return &domain.Session{UserID: 1}, nil
		},
	}
	users := &stubUserLookup{
		getByIDFn: func(_ context.Context, id int) (*domain.User, error) {
			return &domain.User{ID: id, State: 1, Email: "unverified@example.com"}, nil
		},
	}
	h := newVerifyHandler(users, sess, &stubAccountService{}, nil)

	rr := httptest.NewRecorder()
	req := withSessionCookie(httptest.NewRequest(http.MethodGet, "/verify-account", http.NoBody), "ok")
	h.showVerifyAccount(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "unverified@example.com") {
		t.Errorf("body missing user email: %s", body)
	}
	if !strings.Contains(body, "/verify/resend") {
		t.Errorf("body missing resend form action: %s", body)
	}
}

func TestShowVerifyAccount_NoticeQueryParam(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		queryName string
		wantText  string
		wantShown bool
	}{
		{name: "sent", queryName: "sent", wantText: "Verification email sent", wantShown: true},
		{name: "failed", queryName: "failed", wantText: "Please try again in a moment", wantShown: true},
		{name: "unknown notice ignored", queryName: "garbage", wantShown: false},
		{name: "empty notice", queryName: "", wantShown: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sess := &stubSessionService{
				validateFn: func(context.Context, string) (*domain.Session, error) {
					return &domain.Session{UserID: 1}, nil
				},
			}
			users := &stubUserLookup{
				getByIDFn: func(_ context.Context, id int) (*domain.User, error) {
					return &domain.User{ID: id, State: 1, Email: "u@x"}, nil
				},
			}
			h := newVerifyHandler(users, sess, &stubAccountService{}, nil)

			rr := httptest.NewRecorder()
			target := "/verify-account"
			if tt.queryName != "" {
				target += "?notice=" + tt.queryName
			}
			req := withSessionCookie(httptest.NewRequest(http.MethodGet, target, http.NoBody), "ok")
			h.showVerifyAccount(rr, req)

			body := rr.Body.String()
			contains := tt.wantText != "" && strings.Contains(body, tt.wantText)
			if tt.wantShown && !contains {
				t.Errorf("expected %q in body, got: %s", tt.wantText, body)
			}
			if !tt.wantShown {
				for _, notice := range []string{"Verification email sent", "Please try again in a moment"} {
					if strings.Contains(body, notice) {
						t.Errorf("body should not show notice %q for queryName=%q", notice, tt.queryName)
					}
				}
			}
		})
	}
}

func TestDoVerify_EmptyToken_RendersInvalid(t *testing.T) {
	t.Parallel()
	consumeCalled := false
	verify := &stubAccountService{
		consumeVerificationFn: func(context.Context, string) error {
			consumeCalled = true
			return nil
		},
	}
	h := newVerifyHandler(&stubUserLookup{}, &stubSessionService{}, verify, nil)

	rr := httptest.NewRecorder()
	req := postForm("/verify", map[string]string{})
	h.doVerify(rr, req)

	if consumeCalled {
		t.Errorf("ConsumeVerification should not be called for empty token")
	}
	if !strings.Contains(rr.Body.String(), "Invalid link") {
		t.Errorf("body missing invalid result: %s", rr.Body.String())
	}
}

func TestDoVerify_Success_NoSession_RendersSuccess(t *testing.T) {
	t.Parallel()
	verify := &stubAccountService{
		consumeVerificationFn: func(context.Context, string) error { return nil },
	}
	h := newVerifyHandler(&stubUserLookup{}, &stubSessionService{}, verify, nil)

	rr := httptest.NewRecorder()
	req := postForm("/verify", map[string]string{"token": "abc"})
	h.doVerify(rr, req)

	if rr.Code == http.StatusSeeOther {
		t.Errorf("should render page, not redirect; got status %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Email verified") {
		t.Errorf("body missing success message: %s", rr.Body.String())
	}
}

func TestDoVerify_Success_ActiveSession_RedirectsHome(t *testing.T) {
	t.Parallel()
	sess := &stubSessionService{
		validateFn: func(context.Context, string) (*domain.Session, error) {
			return &domain.Session{UserID: 1}, nil
		},
	}
	verify := &stubAccountService{
		consumeVerificationFn: func(context.Context, string) error { return nil },
	}
	h := newVerifyHandler(&stubUserLookup{}, sess, verify, nil)

	rr := httptest.NewRecorder()
	req := withSessionCookie(postForm("/verify", map[string]string{"token": "abc"}), "ok")
	h.doVerify(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusSeeOther)
	}
	if rr.Header().Get("Location") != "/" {
		t.Errorf("Location = %q, want /", rr.Header().Get("Location"))
	}
}

func TestDoVerify_AlreadyUsed_ActiveSession_RedirectsHome(t *testing.T) {
	t.Parallel()
	sess := &stubSessionService{
		validateFn: func(context.Context, string) (*domain.Session, error) {
			return &domain.Session{UserID: 1}, nil
		},
	}
	verify := &stubAccountService{
		consumeVerificationFn: func(context.Context, string) error { return actiontokendomain.ErrTokenAlreadyUsed },
	}
	h := newVerifyHandler(&stubUserLookup{}, sess, verify, nil)

	rr := httptest.NewRecorder()
	req := withSessionCookie(postForm("/verify", map[string]string{"token": "abc"}), "ok")
	h.doVerify(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusSeeOther)
	}
	if rr.Header().Get("Location") != "/" {
		t.Errorf("Location = %q, want /", rr.Header().Get("Location"))
	}
}

func TestDoVerify_AlreadyUsed_NoSession_RendersInvalid(t *testing.T) {
	t.Parallel()
	verify := &stubAccountService{
		consumeVerificationFn: func(context.Context, string) error { return actiontokendomain.ErrTokenAlreadyUsed },
	}
	h := newVerifyHandler(&stubUserLookup{}, &stubSessionService{}, verify, nil)

	rr := httptest.NewRecorder()
	req := postForm("/verify", map[string]string{"token": "abc"})
	h.doVerify(rr, req)

	if rr.Code == http.StatusSeeOther {
		t.Errorf("should render page, not redirect; got status %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Invalid link") {
		t.Errorf("body missing invalid result for already-used + no session: %s", rr.Body.String())
	}
}

func TestDoVerify_Expired_RendersExpired(t *testing.T) {
	t.Parallel()
	verify := &stubAccountService{
		consumeVerificationFn: func(context.Context, string) error { return actiontokendomain.ErrTokenExpired },
	}
	h := newVerifyHandler(&stubUserLookup{}, &stubSessionService{}, verify, nil)

	rr := httptest.NewRecorder()
	req := postForm("/verify", map[string]string{"token": "abc"})
	h.doVerify(rr, req)

	if !strings.Contains(rr.Body.String(), "Link expired") {
		t.Errorf("body missing expired result: %s", rr.Body.String())
	}
}

func TestDoVerify_Invalid_RendersInvalid(t *testing.T) {
	t.Parallel()
	verify := &stubAccountService{
		consumeVerificationFn: func(context.Context, string) error { return actiontokendomain.ErrTokenInvalid },
	}
	h := newVerifyHandler(&stubUserLookup{}, &stubSessionService{}, verify, nil)

	rr := httptest.NewRecorder()
	req := postForm("/verify", map[string]string{"token": "abc"})
	h.doVerify(rr, req)

	if !strings.Contains(rr.Body.String(), "Invalid link") {
		t.Errorf("body missing invalid result: %s", rr.Body.String())
	}
}

func TestDoVerify_GenericError_LogsAndRendersInvalid(t *testing.T) {
	t.Parallel()
	logBuffer := &bytes.Buffer{}
	verify := &stubAccountService{
		consumeVerificationFn: func(context.Context, string) error { return errors.New("db unreachable") },
	}
	h := newVerifyHandler(&stubUserLookup{}, &stubSessionService{}, verify, logBuffer)

	rr := httptest.NewRecorder()
	req := postForm("/verify", map[string]string{"token": "abc"})
	h.doVerify(rr, req)

	if !strings.Contains(rr.Body.String(), "Invalid link") {
		t.Errorf("body missing invalid result: %s", rr.Body.String())
	}
	if !strings.Contains(logBuffer.String(), "verify consume") {
		t.Errorf("expected 'verify consume' in log, got %q", logBuffer.String())
	}
	if !strings.Contains(logBuffer.String(), "db unreachable") {
		t.Errorf("expected underlying error in log, got %q", logBuffer.String())
	}
}

func TestDoVerify_KnownErrorsNotLogged(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err  error
		name string
	}{
		{name: "invalid", err: actiontokendomain.ErrTokenInvalid},
		{name: "already used", err: actiontokendomain.ErrTokenAlreadyUsed},
		{name: "expired", err: actiontokendomain.ErrTokenExpired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			logBuffer := &bytes.Buffer{}
			verify := &stubAccountService{
				consumeVerificationFn: func(context.Context, string) error { return tt.err },
			}
			h := newVerifyHandler(&stubUserLookup{}, &stubSessionService{}, verify, logBuffer)

			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/verify?token=abc", http.NoBody)
			h.doVerify(rr, req)

			if strings.Contains(logBuffer.String(), "verify consume") {
				t.Errorf("expected no error log for %v; got %q", tt.err, logBuffer.String())
			}
		})
	}
}

func TestHasActiveSession(t *testing.T) {
	t.Parallel()

	tests := []struct {
		validate   func(context.Context, string) (*domain.Session, error)
		name       string
		cookieVal  string
		setCookie  bool
		wantActive bool
	}{
		{name: "no cookie", setCookie: false, wantActive: false},
		{name: "empty cookie", setCookie: true, cookieVal: "", wantActive: false},
		{
			name:      "invalid session",
			setCookie: true, cookieVal: "stale",
			validate: func(context.Context, string) (*domain.Session, error) {
				return nil, domain.ErrSessionNotFound
			},
			wantActive: false,
		},
		{
			name:      "valid session",
			setCookie: true, cookieVal: "ok",
			validate: func(context.Context, string) (*domain.Session, error) {
				return &domain.Session{UserID: 1}, nil
			},
			wantActive: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sess := &stubSessionService{validateFn: tt.validate}
			h := newVerifyHandler(&stubUserLookup{}, sess, &stubAccountService{}, nil)

			req := httptest.NewRequest(http.MethodGet, "/anywhere", http.NoBody)
			if tt.setCookie {
				req = withSessionCookie(req, tt.cookieVal)
			}
			if got := h.hasActiveSession(req); got != tt.wantActive {
				t.Errorf("hasActiveSession = %v, want %v", got, tt.wantActive)
			}
		})
	}
}

func TestDoResendVerification_NoCookie_RedirectsToLogin(t *testing.T) {
	t.Parallel()
	h := newVerifyHandler(&stubUserLookup{}, &stubSessionService{}, &stubAccountService{}, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/verify/resend", http.NoBody)
	h.doResendVerification(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusSeeOther)
	}
	if rr.Header().Get("Location") != "/login" {
		t.Errorf("Location = %q, want /login", rr.Header().Get("Location"))
	}
}

func TestDoResendVerification_InvalidSession_RedirectsToLogin(t *testing.T) {
	t.Parallel()
	sess := &stubSessionService{
		validateFn: func(context.Context, string) (*domain.Session, error) {
			return nil, domain.ErrSessionNotFound
		},
	}
	h := newVerifyHandler(&stubUserLookup{}, sess, &stubAccountService{}, nil)

	rr := httptest.NewRecorder()
	req := withSessionCookie(httptest.NewRequest(http.MethodPost, "/verify/resend", http.NoBody), "stale")
	h.doResendVerification(rr, req)

	if rr.Header().Get("Location") != "/login" {
		t.Errorf("Location = %q, want /login", rr.Header().Get("Location"))
	}
}

func TestDoResendVerification_Success_NotifiesSent(t *testing.T) {
	t.Parallel()
	resendCalled := 0
	sess := &stubSessionService{
		validateFn: func(context.Context, string) (*domain.Session, error) {
			return &domain.Session{UserID: 42}, nil
		},
	}
	verify := &stubAccountService{
		resendVerificationFn: func(_ context.Context, accountID int) error {
			resendCalled++
			if accountID != 42 {
				t.Errorf("ResendVerification accountID = %d, want 42", accountID)
			}
			return nil
		},
	}
	h := newVerifyHandler(&stubUserLookup{}, sess, verify, nil)

	rr := httptest.NewRecorder()
	req := withSessionCookie(httptest.NewRequest(http.MethodPost, "/verify/resend", http.NoBody), "ok")
	h.doResendVerification(rr, req)

	if resendCalled != 1 {
		t.Errorf("ResendVerification calls = %d, want 1", resendCalled)
	}
	if rr.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusSeeOther)
	}
	if rr.Header().Get("Location") != "/verify-account?notice=sent" {
		t.Errorf("Location = %q, want /verify-account?notice=sent", rr.Header().Get("Location"))
	}
}

func TestDoResendVerification_Failure_NotifiesFailedAndLogs(t *testing.T) {
	t.Parallel()
	logBuffer := &bytes.Buffer{}
	sess := &stubSessionService{
		validateFn: func(context.Context, string) (*domain.Session, error) {
			return &domain.Session{UserID: 1}, nil
		},
	}
	verify := &stubAccountService{
		resendVerificationFn: func(context.Context, int) error { return errors.New("send boom") },
	}
	h := newVerifyHandler(&stubUserLookup{}, sess, verify, logBuffer)

	rr := httptest.NewRecorder()
	req := withSessionCookie(httptest.NewRequest(http.MethodPost, "/verify/resend", http.NoBody), "ok")
	h.doResendVerification(rr, req)

	if rr.Header().Get("Location") != "/verify-account?notice=failed" {
		t.Errorf("Location = %q, want /verify-account?notice=failed", rr.Header().Get("Location"))
	}
	if !strings.Contains(logBuffer.String(), "verify resend") {
		t.Errorf("expected 'verify resend' in log, got %q", logBuffer.String())
	}
}

func TestDoResendVerification_HTMX_UsesHXRedirectHeader(t *testing.T) {
	t.Parallel()
	sess := &stubSessionService{
		validateFn: func(context.Context, string) (*domain.Session, error) {
			return &domain.Session{UserID: 1}, nil
		},
	}
	h := newVerifyHandler(&stubUserLookup{}, sess, &stubAccountService{}, nil)

	rr := httptest.NewRecorder()
	req := withSessionCookie(httptest.NewRequest(http.MethodPost, "/verify/resend", http.NoBody), "ok")
	req.Header.Set("HX-Request", "true")
	h.doResendVerification(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
	if rr.Header().Get("HX-Redirect") != "/verify-account?notice=sent" {
		t.Errorf("HX-Redirect = %q, want /verify-account?notice=sent", rr.Header().Get("HX-Redirect"))
	}
}
