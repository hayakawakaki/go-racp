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
}

type SessionRepository interface {
	Create(ctx context.Context, s *Session) error
	GetByTokenHash(ctx context.Context, hash [32]byte) (*Session, error)
	Refresh(ctx context.Context, hash [32]byte, lastSeen, expiresAt time.Time) error
	Delete(ctx context.Context, hash [32]byte) error
}
