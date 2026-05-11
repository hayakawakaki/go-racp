package domain

import (
	"database/sql"
	"time"
)

type Action uint8

const (
	ActionUnknown Action = iota
	ActionEmailVerification
)

type ActionToken struct {
	ExpiresAt  time.Time
	CreatedAt  time.Time
	ConsumedAt sql.NullTime
	TokenHash  [32]byte
	AccountID  int
	Action     Action
}

func (t *ActionToken) IsExpired(now time.Time) bool { return !now.Before(t.ExpiresAt) }
func (t *ActionToken) IsConsumed() bool             { return t.ConsumedAt.Valid }
