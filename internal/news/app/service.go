package app

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/hayakawakaki/go-racp/internal/news/domain"
)

type Service struct {
	repo       domain.Repository
	logger     *slog.Logger
	now        func() time.Time
	categories domain.CategoryResolver
}

func NewService(repo domain.Repository, categories domain.CategoryResolver, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}

	return &Service{
		repo:       repo,
		categories: categories,
		logger:     logger,
		now:        time.Now,
	}
}

func (s *Service) Categories() domain.CategoryResolver { return s.categories }

func (s *Service) Create(ctx context.Context, title, body, category string) (int64, error) {
	if err := domain.ValidateTitle(title); err != nil {
		return 0, fmt.Errorf("app.Service.Create: %w", err)
	}
	if err := domain.ValidateBody(body); err != nil {
		return 0, fmt.Errorf("app.Service.Create: %w", err)
	}
	if !s.categories.Has(category) {
		return 0, fmt.Errorf("app.Service.Create: %w", domain.ErrInvalidCategory)
	}

	id, err := s.repo.Create(ctx, domain.News{
		Title:     title,
		Body:      body,
		Category:  category,
		CreatedAt: s.now(),
	})
	if err != nil {
		return 0, fmt.Errorf("app.Service.Create: %w", err)
	}

	return id, nil
}

func (s *Service) Update(ctx context.Context, id int64, title, body, category string) error {
	if err := domain.ValidateTitle(title); err != nil {
		return fmt.Errorf("app.Service.Update: %w", err)
	}
	if err := domain.ValidateBody(body); err != nil {
		return fmt.Errorf("app.Service.Update: %w", err)
	}
	if !s.categories.Has(category) {
		return fmt.Errorf("app.Service.Update: %w", domain.ErrInvalidCategory)
	}

	err := s.repo.Update(ctx, domain.News{
		ID:       id,
		Title:    title,
		Body:     body,
		Category: category,
	})
	if err != nil {
		return fmt.Errorf("app.Service.Update: %w", err)
	}

	return nil
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("app.Service.Delete: %w", err)
	}

	return nil
}

func (s *Service) GetByID(ctx context.Context, id int64) (NewsItem, error) {
	n, err := s.repo.Get(ctx, id)
	if err != nil {
		return NewsItem{}, fmt.Errorf("app.Service.GetByID: %w", err)
	}

	return s.toItem(n), nil
}

func (s *Service) List(ctx context.Context) ([]NewsItem, error) {
	rows, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("app.Service.List: %w", err)
	}

	return s.toItems(rows), nil
}

func (s *Service) ListByCategory(ctx context.Context, category string) ([]NewsItem, error) {
	if !s.categories.Has(category) {
		return nil, fmt.Errorf("app.Service.ListByCategory: %w", domain.ErrInvalidCategory)
	}
	rows, err := s.repo.ListByCategory(ctx, category)
	if err != nil {
		return nil, fmt.Errorf("app.Service.ListByCategory: %w", err)
	}

	return s.toItems(rows), nil
}

func (s *Service) toItem(n domain.News) NewsItem {
	return NewsItem{
		ID:              n.ID,
		Title:           n.Title,
		Body:            n.Body,
		Category:        n.Category,
		CategoryDisplay: s.categories.Display(n.Category),
		CreatedAt:       n.CreatedAt,
	}
}

func (s *Service) toItems(rows []domain.News) []NewsItem {
	out := make([]NewsItem, 0, len(rows))
	for _, n := range rows {
		out = append(out, s.toItem(n))
	}

	return out
}
