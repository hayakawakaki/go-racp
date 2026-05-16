package infra

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hayakawakaki/go-racp/internal/character/domain"
)

type CooldownRepository struct {
	Pool *pgxpool.Pool
}

func NewCooldownRepository(pool *pgxpool.Pool) *CooldownRepository {
	return &CooldownRepository{Pool: pool}
}

func (r *CooldownRepository) Record(ctx context.Context, charID int, t domain.ChangeType, at time.Time) error {
	_, err := r.Pool.Exec(ctx,
		`INSERT INTO cp_character_record (char_id, change_type, changed_at)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (char_id, change_type) DO UPDATE SET changed_at = EXCLUDED.changed_at`,
		charID, t, at,
	)
	if err != nil {
		return fmt.Errorf("infra.CooldownRepository.Record: %w", err)
	}

	return nil
}

func (r *CooldownRepository) MostRecent(ctx context.Context, charID int, t domain.ChangeType) (time.Time, error) {
	var changedAt time.Time
	err := r.Pool.QueryRow(ctx,
		`SELECT changed_at FROM cp_character_record WHERE char_id = $1 AND change_type = $2`,
		charID, t,
	).Scan(&changedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("infra.CooldownRepository.MostRecent: %w", err)
	}

	return changedAt, nil
}
