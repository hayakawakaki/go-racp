package middleware

import (
	"context"
	"time"

	"github.com/hayakawakaki/go-racp/internal/account/domain"
)

type stubSessionService struct {
	validateFn   func(context.Context, string) (*domain.Session, error)
	destroyFn    func(context.Context, string) error
	destroyCalls []string
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

func (s *stubSessionService) TTL() time.Duration { return 24 * time.Hour }

type stubUserLookup struct {
	getByIDFn func(context.Context, int) (*domain.User, error)
}

func (s *stubUserLookup) GetByID(ctx context.Context, id int) (*domain.User, error) {
	if s.getByIDFn != nil {
		return s.getByIDFn(ctx, id)
	}
	return &domain.User{ID: id}, nil
}
