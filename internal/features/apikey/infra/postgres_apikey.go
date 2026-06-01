package infra

import (
	"context"
	"fmt"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/apikey/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const selectColumns = `id, key_hash, name, rate_tier, last_used_at, created_at, revoked_at`

type Repository struct {
	Pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{Pool: pool}
}

func (r *Repository) Create(ctx context.Context, key *domain.APIKey) error {
	err := r.Pool.QueryRow(ctx,
		`INSERT INTO cp_api_keys (key_hash, name, rate_tier) VALUES ($1, $2, $3) RETURNING id, created_at`,
		key.KeyHash, key.Name, key.RateTier,
	).Scan(&key.ID, &key.CreatedAt)
	if err != nil {
		return fmt.Errorf("infra.Repository.Create: %w", err)
	}

	return nil
}

func (r *Repository) List(ctx context.Context) ([]domain.APIKey, error) {
	rows, err := r.Pool.Query(ctx,
		`SELECT `+selectColumns+` FROM cp_api_keys ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("infra.Repository.List: %w", err)
	}
	defer rows.Close()

	out, err := collectAPIKeys(rows)
	if err != nil {
		return nil, fmt.Errorf("infra.Repository.List: %w", err)
	}

	return out, nil
}

func (r *Repository) Revoke(ctx context.Context, id int64) error {
	tag, err := r.Pool.Exec(ctx,
		`UPDATE cp_api_keys SET revoked_at = NOW() WHERE id = $1 AND revoked_at IS NULL`,
		id,
	)
	if err != nil {
		return fmt.Errorf("infra.Repository.Revoke: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("infra.Repository.Revoke: %w", domain.ErrKeyNotFound)
	}

	return nil
}

func (r *Repository) LoadActive(ctx context.Context) ([]domain.APIKey, error) {
	rows, err := r.Pool.Query(ctx,
		`SELECT `+selectColumns+` FROM cp_api_keys WHERE revoked_at IS NULL ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("infra.Repository.LoadActive: %w", err)
	}
	defer rows.Close()

	out, err := collectAPIKeys(rows)
	if err != nil {
		return nil, fmt.Errorf("infra.Repository.LoadActive: %w", err)
	}

	return out, nil
}

func (r *Repository) TouchLastUsed(ctx context.Context, id int64, at time.Time) error {
	_, err := r.Pool.Exec(ctx,
		`UPDATE cp_api_keys SET last_used_at = $1 WHERE id = $2`,
		at, id,
	)
	if err != nil {
		return fmt.Errorf("infra.Repository.TouchLastUsed: %w", err)
	}

	return nil
}

func scanAPIKey(row pgx.Row) (domain.APIKey, error) {
	var key domain.APIKey
	err := row.Scan(&key.ID, &key.KeyHash, &key.Name, &key.RateTier, &key.LastUsedAt, &key.CreatedAt, &key.RevokedAt)
	if err != nil {
		return domain.APIKey{}, fmt.Errorf("infra.scanAPIKey: %w", err)
	}

	return key, nil
}

func collectAPIKeys(rows pgx.Rows) ([]domain.APIKey, error) {
	out := make([]domain.APIKey, 0)
	for rows.Next() {
		key, err := scanAPIKey(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, key)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("infra.collectAPIKeys: %w", err)
	}

	return out, nil
}
