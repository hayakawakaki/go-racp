package middleware

import (
	"context"
	"time"

	"github.com/hayakawakaki/go-racp/internal/account/app"
	"github.com/hayakawakaki/go-racp/internal/account/domain"
)

type UserLookup interface {
	GetByID(ctx context.Context, id int) (*domain.User, error)
}

type snapshotKey int

const accountSnapshotKey snapshotKey = 0

type AccountSnapshot struct {
	UnbanTime time.Time
	UserID    int
	Tier      app.Tier
}

func ContextWithSnapshot(ctx context.Context, snap *AccountSnapshot) context.Context {
	return context.WithValue(ctx, accountSnapshotKey, snap)
}

func SnapshotFromContext(ctx context.Context) (*AccountSnapshot, bool) {
	snap, ok := ctx.Value(accountSnapshotKey).(*AccountSnapshot)
	return snap, ok
}
