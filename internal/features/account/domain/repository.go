package domain

import (
	"context"
	"time"
)

type Repository interface {
	Create(ctx context.Context, user *User, password string) (*User, error)
	GetAll(ctx context.Context) ([]User, error)
	GetByID(ctx context.Context, id int) (*User, error)
	GetByUsername(ctx context.Context, username string) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	Delete(ctx context.Context, id int) error
	GetCredentials(ctx context.Context, username string) (*User, string, error)
	VerifyPassword(ctx context.Context, id int, password string) (bool, error)
	MarkVerified(ctx context.Context, accountID int) error
	UpdatePassword(ctx context.Context, accountID int, newPassword string) error
	UpdateEmail(ctx context.Context, accountID int, newEmail string) error
}

type SessionRepository interface {
	Create(ctx context.Context, s *Session) error
	GetByTokenHash(ctx context.Context, hash [32]byte) (*Session, error)
	Refresh(ctx context.Context, hash [32]byte, lastSeen, expiresAt time.Time) error
	Delete(ctx context.Context, hash [32]byte) error
	DeleteByUserID(ctx context.Context, userID int) error
	DeleteByUserIDExcept(ctx context.Context, userID int, exceptHash [32]byte) error
}
