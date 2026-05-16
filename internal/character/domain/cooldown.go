package domain

import (
	"context"
	"time"
)

type ChangeType uint8

const (
	ChangeTypeUnknown ChangeType = iota
	ChangeTypeLook
	ChangeTypeLocation
)

type Cooldowns interface {
	Record(ctx context.Context, charID int, t ChangeType, at time.Time) error
	MostRecent(ctx context.Context, charID int, t ChangeType) (time.Time, error)
}
