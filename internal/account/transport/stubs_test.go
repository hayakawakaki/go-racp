package transport

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	accountapp "github.com/hayakawakaki/go-racp/internal/account/app"
	authdomain "github.com/hayakawakaki/go-racp/internal/auth/domain"
	authtransport "github.com/hayakawakaki/go-racp/internal/auth/transport"
)

const stubSessionTTL = 24 * time.Hour

func newTestHandler(svc accountService, sess sessionService, logBuffer io.Writer) *Handler {
	if logBuffer == nil {
		logBuffer = io.Discard
	}
	return &Handler{
		svc:     svc,
		sessSvc: sess,
		logger:  slog.New(slog.NewTextHandler(logBuffer, nil)),
	}
}

func reqWithSession(method, target string, userID int, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	ctx := authtransport.ContextWithSession(req.Context(), &authdomain.Session{UserID: userID})
	return req.WithContext(ctx)
}

func postWithSession(target string, userID int, values map[string]string) *http.Request {
	req := reqWithSession(http.MethodPost, target, userID, strings.NewReader(encodeForm(values)))
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

type stubAccountService struct {
	getAccountFn         func(context.Context, int) (*accountapp.AccountDTO, error)
	updatePasswordFn     func(context.Context, int, string, string, string, string) error
	requestEmailChangeFn func(context.Context, int, string, string) error
	consumeEmailChangeFn func(context.Context, string) (*authdomain.User, error)
	updatePasswordCalls  []updatePasswordCall
	requestEmailCalls    []requestEmailCall
}

type updatePasswordCall struct {
	CurrentRawToken string
	CurrentPassword string
	NewPassword     string
	ConfirmPassword string
	UserID          int
}

type requestEmailCall struct {
	CurrentPassword string
	NewEmail        string
	UserID          int
}

func (s *stubAccountService) GetAccount(ctx context.Context, userID int) (*accountapp.AccountDTO, error) {
	if s.getAccountFn != nil {
		return s.getAccountFn(ctx, userID)
	}
	return &accountapp.AccountDTO{Username: "u", Email: "u@x", Verified: true}, nil
}

func (s *stubAccountService) UpdatePassword(ctx context.Context, userID int, currentRawToken, currentPassword, newPassword, confirmPassword string) error {
	s.updatePasswordCalls = append(s.updatePasswordCalls, updatePasswordCall{
		CurrentRawToken: currentRawToken,
		CurrentPassword: currentPassword,
		NewPassword:     newPassword,
		ConfirmPassword: confirmPassword,
		UserID:          userID,
	})
	if s.updatePasswordFn != nil {
		return s.updatePasswordFn(ctx, userID, currentRawToken, currentPassword, newPassword, confirmPassword)
	}
	return nil
}

func (s *stubAccountService) RequestEmailChange(ctx context.Context, userID int, currentPassword, newEmail string) error {
	s.requestEmailCalls = append(s.requestEmailCalls, requestEmailCall{
		CurrentPassword: currentPassword,
		NewEmail:        newEmail,
		UserID:          userID,
	})
	if s.requestEmailChangeFn != nil {
		return s.requestEmailChangeFn(ctx, userID, currentPassword, newEmail)
	}
	return nil
}

func (s *stubAccountService) ConsumeEmailChange(ctx context.Context, rawToken string) (*authdomain.User, error) {
	if s.consumeEmailChangeFn != nil {
		return s.consumeEmailChangeFn(ctx, rawToken)
	}
	return &authdomain.User{Email: "new@example.com"}, nil
}

type stubSessionService struct {
	validateFn func(context.Context, string) (*authdomain.Session, error)
}

func (s *stubSessionService) Validate(ctx context.Context, rawToken string) (*authdomain.Session, error) {
	if s.validateFn != nil {
		return s.validateFn(ctx, rawToken)
	}
	return nil, authdomain.ErrSessionNotFound
}

func (s *stubSessionService) TTL() time.Duration { return stubSessionTTL }
