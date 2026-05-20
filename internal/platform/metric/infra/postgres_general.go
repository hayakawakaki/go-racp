package infra

import (
	"context"
	"errors"
	"fmt"

	"github.com/hayakawakaki/go-racp/internal/platform/metric/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type GeneralRepository struct {
	Pool *pgxpool.Pool
}

func NewGeneralRepository(pool *pgxpool.Pool) *GeneralRepository {
	return &GeneralRepository{Pool: pool}
}

func (r *GeneralRepository) Insert(ctx context.Context, snap domain.GeneralSnapshot) error {
	_, err := r.Pool.Exec(ctx,
		`INSERT INTO cp_metric_general_snapshot (total_accounts, total_characters, total_guilds)
		 VALUES ($1, $2, $3)`,
		snap.TotalAccounts, snap.TotalCharacters, snap.TotalGuilds,
	)
	if err != nil {
		return fmt.Errorf("infra.GeneralRepository.Insert: %w", err)
	}
	return nil
}

func (r *GeneralRepository) Latest(ctx context.Context) (domain.GeneralSnapshot, error) {
	var snap domain.GeneralSnapshot
	err := r.Pool.QueryRow(ctx,
		`SELECT captured_at, total_accounts, total_characters, total_guilds
		 FROM cp_metric_general_snapshot
		 ORDER BY captured_at DESC LIMIT 1`,
	).Scan(&snap.CapturedAt, &snap.TotalAccounts, &snap.TotalCharacters, &snap.TotalGuilds)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.GeneralSnapshot{}, nil
	}
	if err != nil {
		return domain.GeneralSnapshot{}, fmt.Errorf("infra.GeneralRepository.Latest: %w", err)
	}
	return snap, nil
}
