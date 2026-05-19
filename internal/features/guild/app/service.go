package app

import (
	"context"
	"fmt"

	"github.com/hayakawakaki/go-racp/internal/features/guild/domain"
)

type Service struct {
	repo domain.Repository
}

func NewService(repo domain.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(ctx context.Context, q ListQuery) (GuildPage, error) {
	page, err := s.repo.List(ctx, q)
	if err != nil {
		return GuildPage{}, fmt.Errorf("app.Service.List: %w", err)
	}

	return page, nil
}

func (s *Service) Get(ctx context.Context, id int) (GuildDetail, error) {
	g, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return GuildDetail{}, fmt.Errorf("app.Service.Get: %w", err)
	}
	members, err := s.repo.ListMembers(ctx, id)
	if err != nil {
		return GuildDetail{}, fmt.Errorf("app.Service.Get members: %w", err)
	}

	return GuildDetail{Guild: g, Members: members}, nil
}

func (s *Service) GetEmblem(ctx context.Context, id int) (data []byte, mime string, err error) {
	data, mime, err = s.repo.GetEmblem(ctx, id)
	if err != nil {
		return nil, "", fmt.Errorf("app.Service.GetEmblem: %w", err)
	}

	return data, mime, nil
}
