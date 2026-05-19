package infra

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/hayakawakaki/go-racp/internal/platform/actiontoken/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	Pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{Pool: pool}
}

func (r *PostgresRepository) Insert(ctx context.Context, t *domain.ActionToken) error {
	_, err := r.Pool.Exec(ctx,
		`INSERT INTO cp_action_tokens (token_hash, account_id, action, expires_at, consumed_at, created_at, payload)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		t.TokenHash[:], t.AccountID, t.Action, t.ExpiresAt, t.ConsumedAt, t.CreatedAt, t.Payload,
	)
	if err != nil {
		return fmt.Errorf("infra.PostgresRepository.Insert: %w", err)
	}

	return nil
}

func (r *PostgresRepository) GetByHash(ctx context.Context, hash [32]byte) (*domain.ActionToken, error) {
	var (
		t   domain.ActionToken
		raw []byte
	)
	err := r.Pool.QueryRow(ctx,
		`SELECT token_hash, account_id, action, expires_at, consumed_at, created_at, payload
		 FROM cp_action_tokens WHERE token_hash = $1`, hash[:],
	).Scan(&raw, &t.AccountID, &t.Action, &t.ExpiresAt, &t.ConsumedAt, &t.CreatedAt, &t.Payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrTokenInvalid
	}
	if err != nil {
		return nil, fmt.Errorf("infra.PostgresRepository.GetByHash: %w", err)
	}
	if len(raw) != 32 {
		return nil, fmt.Errorf("infra.PostgresRepository.GetByHash: token_hash len=%d", len(raw))
	}
	copy(t.TokenHash[:], raw)

	return &t, nil
}

func (r *PostgresRepository) DeleteUnconsumed(ctx context.Context, accountID int, action domain.Action) error {
	_, err := r.Pool.Exec(ctx,
		`DELETE FROM cp_action_tokens WHERE account_id = $1 AND action = $2 AND consumed_at IS NULL`,
		accountID, action,
	)
	if err != nil {
		return fmt.Errorf("infra.PostgresRepository.DeleteUnconsumed: %w", err)
	}

	return nil
}

func (r *PostgresRepository) MarkConsumed(ctx context.Context, hash [32]byte, at time.Time) error {
	tag, err := r.Pool.Exec(ctx,
		`UPDATE cp_action_tokens SET consumed_at = $1 WHERE token_hash = $2 AND consumed_at IS NULL`,
		at, hash[:],
	)
	if err != nil {
		return fmt.Errorf("infra.PostgresRepository.MarkConsumed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrTokenAlreadyUsed
	}

	return nil
}

func (r *PostgresRepository) MostRecentIssuedAt(ctx context.Context, accountID int, action domain.Action) (time.Time, error) {
	var t sql.NullTime
	err := r.Pool.QueryRow(ctx,
		`SELECT MAX(created_at) FROM cp_action_tokens WHERE account_id = $1 AND action = $2`,
		accountID, action,
	).Scan(&t)
	if err != nil {
		return time.Time{}, fmt.Errorf("infra.PostgresRepository.MostRecentIssuedAt: %w", err)
	}
	if !t.Valid {
		return time.Time{}, nil
	}

	return t.Time, nil
}
