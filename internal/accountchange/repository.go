package accountchange

import (
	"context"
	"time"
)

type Repository interface {
	Record(ctx context.Context, accountID int, changeType Type, at time.Time) error
	MostRecent(ctx context.Context, accountID int, changeType Type) (time.Time, error)
}
