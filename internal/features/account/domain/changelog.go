package domain

import (
	"context"
	"time"
)

type ChangeType uint8

const (
	ChangeTypeUnknown ChangeType = iota
	ChangeTypePassword
	ChangeTypeEmail
)

type ChangeLog interface {
	Record(ctx context.Context, accountID int, changeType ChangeType, at time.Time) error
	MostRecent(ctx context.Context, accountID int, changeType ChangeType) (time.Time, error)
}
