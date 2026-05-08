package transport

import (
	"context"

	"github.com/hayakawakaki/go-racp/internal/auth/app"
	"github.com/hayakawakaki/go-racp/internal/auth/domain"
)

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
