package domain

import "time"

type APIKey struct {
	CreatedAt  time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
	Name       string
	RateTier   string
	KeyHash    []byte
	ID         int64
}

func (k APIKey) IsRevoked() bool {
	return k.RevokedAt != nil
}
