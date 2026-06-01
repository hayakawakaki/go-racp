package domain

import (
	"context"
	"time"
)

type Repository interface {
	Create(ctx context.Context, key *APIKey) error
	List(ctx context.Context) ([]APIKey, error)
	Revoke(ctx context.Context, id int64) error
	LoadActive(ctx context.Context) ([]APIKey, error)
	TouchLastUsed(ctx context.Context, id int64, at time.Time) error
}
