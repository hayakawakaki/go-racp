package infra

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hayakawakaki/go-racp/internal/account/domain"
)

type ChangeLogRepository struct {
	Pool *pgxpool.Pool
}

func NewChangeLogRepository(pool *pgxpool.Pool) *ChangeLogRepository {
	return &ChangeLogRepository{Pool: pool}
}

func (r *ChangeLogRepository) Record(ctx context.Context, accountID int, changeType domain.ChangeType, at time.Time) error {
	_, err := r.Pool.Exec(ctx,
		`INSERT INTO cp_account_record (account_id, change_type, changed_at)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (account_id, change_type) DO UPDATE SET changed_at = EXCLUDED.changed_at`,
		accountID, changeType, at,
	)
	if err != nil {
		return fmt.Errorf("infra.ChangeLogRepository.Record: %w", err)
	}

	return nil
}

func (r *ChangeLogRepository) MostRecent(ctx context.Context, accountID int, changeType domain.ChangeType) (time.Time, error) {
	var changedAt time.Time
	err := r.Pool.QueryRow(ctx,
		`SELECT changed_at FROM cp_account_record WHERE account_id = $1 AND change_type = $2`,
		accountID, changeType,
	).Scan(&changedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("infra.ChangeLogRepository.MostRecent: %w", err)
	}

	return changedAt, nil
}
