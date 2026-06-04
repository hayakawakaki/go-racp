package domain

import (
	"context"
	"time"
)

type Notification struct {
	CreatedAt time.Time
	ReadAt    *time.Time
	Category  string
	Title     string
	Body      string
	Link      string
	ID        int64
	AccountID int
}

func (n Notification) IsRead() bool {
	return n.ReadAt != nil
}

type Repository interface {
	Create(ctx context.Context, notification Notification) (Notification, error)
	RecentByAccount(ctx context.Context, accountID, limit int) ([]Notification, error)
	UnreadCount(ctx context.Context, accountID int) (int, error)
	MarkRead(ctx context.Context, accountID int, id int64, now time.Time) (string, error)
	MarkAllRead(ctx context.Context, accountID int, now time.Time) (int64, error)
	PruneOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}
