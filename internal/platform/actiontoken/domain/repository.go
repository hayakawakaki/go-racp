package domain

import (
	"context"
	"time"
)

type Repository interface {
	Insert(ctx context.Context, t *ActionToken) error
	GetByHash(ctx context.Context, hash [32]byte) (*ActionToken, error)
	DeleteUnconsumed(ctx context.Context, accountID int, action Action) error
	MarkConsumed(ctx context.Context, hash [32]byte, at time.Time) error
	MostRecentIssuedAt(ctx context.Context, accountID int, action Action) (time.Time, error)
}
