package domain

import "time"

type Session struct {
	ExpiresAt  time.Time
	CreatedAt  time.Time
	LastSeenAt time.Time
	TokenHash  [32]byte
	UserID     int
}

func (s *Session) IsExpired(now time.Time) bool {
	return !now.Before(s.ExpiresAt)
}
