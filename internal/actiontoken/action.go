package actiontoken

import (
	"database/sql"
	"time"
)

type Action uint8

const (
	Unknown Action = iota
	EmailVerification
	PasswordReset
	EmailChange
)

type ActionToken struct {
	ExpiresAt  time.Time
	CreatedAt  time.Time
	ConsumedAt sql.NullTime
	Payload    []byte
	TokenHash  [32]byte
	AccountID  int
	Action     Action
}

func (t *ActionToken) IsExpired(now time.Time) bool { return !now.Before(t.ExpiresAt) }
func (t *ActionToken) IsConsumed() bool             { return t.ConsumedAt.Valid }
