package middleware

import (
	"context"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
)

const (
	NoticeBanned     = "banned"
	NoticeDeleted    = "deleted"
	NoticeBanBlocked = "ban_blocked"
)

type AuthPolicy struct {
	AllowTempBannedLogin bool
	Unrestricted         bool
}

type UserLookup interface {
	GetByID(ctx context.Context, id int) (*domain.User, error)
}

type snapshotKey int

const accountSnapshotKey snapshotKey = 0

type AccountSnapshot struct {
	UnbanTime time.Time
	Username  string
	UserID    int
	GroupID   int
	State     int
}

func (s *AccountSnapshot) IsAdmin() bool {
	return s.GroupID == domain.RoleAdmin.GroupID
}

func ContextWithSnapshot(ctx context.Context, snap *AccountSnapshot) context.Context {
	return context.WithValue(ctx, accountSnapshotKey, snap)
}

func SnapshotFromContext(ctx context.Context) (*AccountSnapshot, bool) {
	snap, ok := ctx.Value(accountSnapshotKey).(*AccountSnapshot)
	return snap, ok
}
