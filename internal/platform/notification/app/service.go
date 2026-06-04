package app

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/hayakawakaki/go-racp/internal/platform/notification/domain"
)

const inboxPageSize = 50

type Page struct {
	Items      []domain.Notification
	Total      int
	Page       int
	PerPage    int
	TotalPages int
}

type Service struct {
	repo        domain.Repository
	broadcaster *Broadcaster
	logger      *slog.Logger
	now         func() time.Time
	recentLimit int
	retention   time.Duration
}

type Option func(*Service)

func WithNow(fn func() time.Time) Option {
	return func(s *Service) {
		if fn != nil {
			s.now = fn
		}
	}
}

func WithRecentLimit(limit int) Option {
	return func(s *Service) {
		if limit > 0 {
			s.recentLimit = limit
		}
	}
}

func WithRetention(d time.Duration) Option {
	return func(s *Service) { s.retention = d }
}

func NewService(repo domain.Repository, broadcaster *Broadcaster, logger *slog.Logger, opts ...Option) *Service {
	s := &Service{
		repo:        repo,
		broadcaster: broadcaster,
		logger:      logger,
		now:         time.Now,
		recentLimit: 20,
	}
	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *Service) Emit(ctx context.Context, accountID int, category, title, body, link string) error {
	if accountID <= 0 {
		return domain.ErrInvalidAccount
	}

	created, err := s.repo.Create(ctx, domain.Notification{
		AccountID: accountID,
		Category:  category,
		Title:     title,
		Body:      body,
		Link:      link,
	})
	if err != nil {
		return fmt.Errorf("notification.Service.Emit: %w", err)
	}

	unread, err := s.repo.UnreadCount(ctx, accountID)
	if err != nil {
		return fmt.Errorf("notification.Service.Emit: %w", err)
	}

	s.broadcaster.Publish(accountID, Event{Notification: created, Unread: unread})

	return nil
}

func (s *Service) Recent(ctx context.Context, accountID int) ([]domain.Notification, error) {
	items, err := s.repo.RecentByAccount(ctx, accountID, s.recentLimit)
	if err != nil {
		return nil, fmt.Errorf("notification.Service.Recent: %w", err)
	}

	return items, nil
}

func (s *Service) Inbox(ctx context.Context, accountID int, unreadOnly bool, page int) (Page, error) {
	page = max(page, 1)

	offset := (page - 1) * inboxPageSize

	items, total, err := s.repo.ListPage(ctx, accountID, unreadOnly, inboxPageSize, offset)
	if err != nil {
		return Page{}, fmt.Errorf("notification.Service.Inbox: %w", err)
	}

	totalPages := max((total+inboxPageSize-1)/inboxPageSize, 1)

	if page > totalPages {
		page = totalPages
		offset = (page - 1) * inboxPageSize

		items, _, err = s.repo.ListPage(ctx, accountID, unreadOnly, inboxPageSize, offset)
		if err != nil {
			return Page{}, fmt.Errorf("notification.Service.Inbox: %w", err)
		}
	}

	return Page{
		Items:      items,
		Total:      total,
		Page:       page,
		PerPage:    inboxPageSize,
		TotalPages: totalPages,
	}, nil
}

func (s *Service) UnreadCount(ctx context.Context, accountID int) (int, error) {
	count, err := s.repo.UnreadCount(ctx, accountID)
	if err != nil {
		return 0, fmt.Errorf("notification.Service.UnreadCount: %w", err)
	}

	return count, nil
}

func (s *Service) MarkRead(ctx context.Context, accountID int, id int64) (string, error) {
	link, err := s.repo.MarkRead(ctx, accountID, id, s.now())
	if err != nil {
		return "", fmt.Errorf("notification.Service.MarkRead: %w", err)
	}

	s.PublishUnread(ctx, accountID)

	return link, nil
}

func (s *Service) MarkAllRead(ctx context.Context, accountID int) error {
	if _, err := s.repo.MarkAllRead(ctx, accountID, s.now()); err != nil {
		return fmt.Errorf("notification.Service.MarkAllRead: %w", err)
	}

	s.PublishUnread(ctx, accountID)

	return nil
}

func (s *Service) PublishUnread(ctx context.Context, accountID int) {
	unread, err := s.repo.UnreadCount(ctx, accountID)
	if err != nil {
		s.logger.Warn("notification: publish unread", "err", err)
		return
	}

	s.broadcaster.Publish(accountID, Event{Unread: unread})
}

func (s *Service) Subscribe(accountID int) (events <-chan Event, cancel func()) {
	return s.broadcaster.Subscribe(accountID)
}

func (s *Service) Prune(ctx context.Context) (int64, error) {
	if s.retention <= 0 {
		return 0, nil
	}

	removed, err := s.repo.PruneOlderThan(ctx, s.now().Add(-s.retention))
	if err != nil {
		return 0, fmt.Errorf("notification.Service.Prune: %w", err)
	}

	return removed, nil
}
