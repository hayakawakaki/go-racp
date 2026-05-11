package domain

import (
	"context"
	"time"
)

type Repository interface {
	Create(ctx context.Context, user *User) (*User, error)
	GetAll(ctx context.Context) ([]User, error)
	GetByID(ctx context.Context, id int) (*User, error)
	GetByUsername(ctx context.Context, username string) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	Update(ctx context.Context, user *User) (*User, error)
	Delete(ctx context.Context, id int) error
	Authenticate(ctx context.Context, username, password string) (*User, error)
	MarkVerified(ctx context.Context, accountID int) error
}

type SessionRepository interface {
	Create(ctx context.Context, s *Session) error
	GetByTokenHash(ctx context.Context, hash [32]byte) (*Session, error)
	Refresh(ctx context.Context, hash [32]byte, lastSeen, expiresAt time.Time) error
	Delete(ctx context.Context, hash [32]byte) error
}

type TokenRepository interface {
	Insert(ctx context.Context, t *ActionToken) error
	GetByHash(ctx context.Context, hash [32]byte) (*ActionToken, error)
	DeleteUnconsumed(ctx context.Context, accountID int, action Action) error
	MarkConsumed(ctx context.Context, hash [32]byte, at time.Time) error
	MostRecentIssuedAt(ctx context.Context, accountID int, action Action) (time.Time, error)
}
