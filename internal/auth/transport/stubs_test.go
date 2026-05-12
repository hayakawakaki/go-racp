package transport

import (
	"context"
	"time"

	"github.com/hayakawakaki/go-racp/internal/actiontoken"
	"github.com/hayakawakaki/go-racp/internal/auth/app"
	"github.com/hayakawakaki/go-racp/internal/auth/domain"
)

const stubSessionTTL = 24 * time.Hour

type stubAuthService struct {
	createFn func(context.Context, app.CreateCommand) (*app.GetDTO, error)
	authNFn  func(context.Context, app.LoginCommand) (*app.GetDTO, error)
}

func (s *stubAuthService) Create(ctx context.Context, cmd app.CreateCommand) (*app.GetDTO, error) {
	if s.createFn != nil {
		return s.createFn(ctx, cmd)
	}
	return &app.GetDTO{ID: 1, Username: cmd.Username, Email: cmd.Email}, nil
}

func (s *stubAuthService) Authenticate(ctx context.Context, cmd app.LoginCommand) (*app.GetDTO, error) {
	if s.authNFn != nil {
		return s.authNFn(ctx, cmd)
	}
	return &app.GetDTO{ID: 1, Username: cmd.Username}, nil
}

type stubSessionService struct {
	createFn     func(context.Context, int) (string, *domain.Session, error)
	validateFn   func(context.Context, string) (*domain.Session, error)
	destroyFn    func(context.Context, string) error
	createCalls  []int
	destroyCalls []string
}

func (s *stubSessionService) Create(ctx context.Context, userID int) (string, *domain.Session, error) {
	s.createCalls = append(s.createCalls, userID)
	if s.createFn != nil {
		return s.createFn(ctx, userID)
	}
	return "stub-token", &domain.Session{UserID: userID}, nil
}

func (s *stubSessionService) Validate(ctx context.Context, rawToken string) (*domain.Session, error) {
	if s.validateFn != nil {
		return s.validateFn(ctx, rawToken)
	}
	return nil, domain.ErrSessionNotFound
}

func (s *stubSessionService) Destroy(ctx context.Context, rawToken string) error {
	s.destroyCalls = append(s.destroyCalls, rawToken)
	if s.destroyFn != nil {
		return s.destroyFn(ctx, rawToken)
	}
	return nil
}

func (s *stubSessionService) TTL() time.Duration { return stubSessionTTL }

type stubUserLookup struct {
	getByIDFn func(context.Context, int) (*domain.User, error)
}

func (s *stubUserLookup) GetByID(ctx context.Context, id int) (*domain.User, error) {
	if s.getByIDFn != nil {
		return s.getByIDFn(ctx, id)
	}
	return &domain.User{ID: id}, nil
}

type stubVerifyService struct {
	consumeFn func(context.Context, string) error
	resendFn  func(context.Context, int) error
}

func (s *stubVerifyService) ConsumeVerification(ctx context.Context, rawToken string) error {
	if s.consumeFn != nil {
		return s.consumeFn(ctx, rawToken)
	}
	return nil
}

func (s *stubVerifyService) ResendVerification(ctx context.Context, accountID int) error {
	if s.resendFn != nil {
		return s.resendFn(ctx, accountID)
	}
	return nil
}

type stubResetService struct {
	requestFn func(context.Context, string) error
	consumeFn func(context.Context, string, string) error
	peekFn    func(context.Context, string) (*actiontoken.ActionToken, error)
}

func (s *stubResetService) RequestPasswordReset(ctx context.Context, email string) error {
	if s.requestFn != nil {
		return s.requestFn(ctx, email)
	}
	return nil
}

func (s *stubResetService) ConsumePasswordReset(ctx context.Context, rawToken, newPassword string) error {
	if s.consumeFn != nil {
		return s.consumeFn(ctx, rawToken, newPassword)
	}
	return nil
}

func (s *stubResetService) PeekPasswordReset(ctx context.Context, rawToken string) (*actiontoken.ActionToken, error) {
	if s.peekFn != nil {
		return s.peekFn(ctx, rawToken)
	}
	return &actiontoken.ActionToken{}, nil
}
