package actiontoken

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type MySQLRepository struct {
	Client *sql.DB
}

func NewMySQLRepository(client *sql.DB) *MySQLRepository {
	return &MySQLRepository{Client: client}
}

func (r *MySQLRepository) Insert(ctx context.Context, t *ActionToken) error {
	_, err := r.Client.ExecContext(ctx,
		`INSERT INTO cp_action_tokens (token_hash, account_id, action, expires_at, consumed_at, created_at, payload)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		t.TokenHash[:], t.AccountID, t.Action, t.ExpiresAt, t.ConsumedAt, t.CreatedAt, t.Payload,
	)
	if err != nil {
		return fmt.Errorf("actiontoken.MySQLRepository.Insert: %w", err)
	}
	return nil
}

func (r *MySQLRepository) GetByHash(ctx context.Context, hash [32]byte) (*ActionToken, error) {
	var (
		t   ActionToken
		raw []byte
	)
	err := r.Client.QueryRowContext(ctx,
		`SELECT token_hash, account_id, action, expires_at, consumed_at, created_at, payload
		 FROM cp_action_tokens WHERE token_hash = ?`, hash[:],
	).Scan(&raw, &t.AccountID, &t.Action, &t.ExpiresAt, &t.ConsumedAt, &t.CreatedAt, &t.Payload)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrTokenInvalid
	}
	if err != nil {
		return nil, fmt.Errorf("actiontoken.MySQLRepository.GetByHash: %w", err)
	}
	if len(raw) != 32 {
		return nil, fmt.Errorf("actiontoken.MySQLRepository.GetByHash: token_hash len=%d", len(raw))
	}
	copy(t.TokenHash[:], raw)
	return &t, nil
}

func (r *MySQLRepository) DeleteUnconsumed(ctx context.Context, accountID int, action Action) error {
	_, err := r.Client.ExecContext(ctx,
		`DELETE FROM cp_action_tokens WHERE account_id = ? AND action = ? AND consumed_at IS NULL`,
		accountID, action,
	)
	if err != nil {
		return fmt.Errorf("actiontoken.MySQLRepository.DeleteUnconsumed: %w", err)
	}
	return nil
}

func (r *MySQLRepository) MarkConsumed(ctx context.Context, hash [32]byte, at time.Time) error {
	res, err := r.Client.ExecContext(ctx,
		`UPDATE cp_action_tokens SET consumed_at = ? WHERE token_hash = ? AND consumed_at IS NULL`,
		at, hash[:],
	)
	if err != nil {
		return fmt.Errorf("actiontoken.MySQLRepository.MarkConsumed: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("actiontoken.MySQLRepository.MarkConsumed: %w", err)
	}
	if rows == 0 {
		return ErrTokenAlreadyUsed
	}
	return nil
}

func (r *MySQLRepository) MostRecentIssuedAt(ctx context.Context, accountID int, action Action) (time.Time, error) {
	var t sql.NullTime
	err := r.Client.QueryRowContext(ctx,
		`SELECT MAX(created_at) FROM cp_action_tokens WHERE account_id = ? AND action = ?`,
		accountID, action,
	).Scan(&t)
	if err != nil {
		return time.Time{}, fmt.Errorf("actiontoken.MySQLRepository.MostRecentIssuedAt: %w", err)
	}
	if !t.Valid {
		return time.Time{}, nil
	}
	return t.Time, nil
}
