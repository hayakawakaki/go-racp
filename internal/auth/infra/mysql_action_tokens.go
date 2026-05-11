package infra

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/hayakawakaki/go-racp/internal/auth/domain"
)

type TokenRepository struct {
	Client *sql.DB
}

func NewTokenRepository(client *sql.DB) *TokenRepository {
	return &TokenRepository{Client: client}
}

func (r *TokenRepository) Insert(ctx context.Context, token *domain.ActionToken) error {
	_, err := r.Client.ExecContext(ctx,
		`INSERT INTO cp_action_tokens (token_hash, account_id, action, expires_at, consumed_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		token.TokenHash[:], token.AccountID, token.Action, token.ExpiresAt, token.ConsumedAt, token.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("infra.TokenRepository.Insert: %w", err)
	}
	return nil
}

func (r *TokenRepository) GetByHash(ctx context.Context, hash [32]byte) (*domain.ActionToken, error) {
	var (
		token   domain.ActionToken
		rawHash []byte
	)
	err := r.Client.QueryRowContext(ctx,
		`SELECT token_hash, account_id, action, expires_at, consumed_at, created_at
		 FROM cp_action_tokens WHERE token_hash = ?`, hash[:],
	).Scan(&rawHash, &token.AccountID, &token.Action, &token.ExpiresAt, &token.ConsumedAt, &token.CreatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrTokenInvalid
	}
	if err != nil {
		return nil, fmt.Errorf("infra.TokenRepository.GetByHash: %w", err)
	}
	if len(rawHash) != 32 {
		return nil, fmt.Errorf("infra.TokenRepository.GetByHash: token_hash len=%d", len(rawHash))
	}
	copy(token.TokenHash[:], rawHash)
	return &token, nil
}

func (r *TokenRepository) DeleteUnconsumed(ctx context.Context, accountID int, action domain.Action) error {
	_, err := r.Client.ExecContext(ctx,
		`DELETE FROM cp_action_tokens WHERE account_id = ? AND action = ? AND consumed_at IS NULL`,
		accountID, action,
	)
	if err != nil {
		return fmt.Errorf("infra.TokenRepository.DeleteUnconsumed: %w", err)
	}
	return nil
}

func (r *TokenRepository) MarkConsumed(ctx context.Context, hash [32]byte, at time.Time) error {
	res, err := r.Client.ExecContext(ctx,
		`UPDATE cp_action_tokens SET consumed_at = ? WHERE token_hash = ? AND consumed_at IS NULL`,
		at, hash[:],
	)
	if err != nil {
		return fmt.Errorf("infra.TokenRepository.MarkConsumed: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("infra.TokenRepository.MarkConsumed: %w", err)
	}
	if rows == 0 {
		var consumedAt sql.NullTime
		err := r.Client.QueryRowContext(ctx,
			`SELECT consumed_at FROM cp_action_tokens WHERE token_hash = ?`,
			hash[:],
		).Scan(&consumedAt)
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrTokenInvalid
		}
		if err != nil {
			return fmt.Errorf("infra.TokenRepository.MarkConsumed: %w", err)
		}
		return domain.ErrTokenAlreadyUsed
	}
	return nil
}

func (r *TokenRepository) MostRecentIssuedAt(ctx context.Context, accountID int, action domain.Action) (time.Time, error) {
	var lastIssued sql.NullTime
	err := r.Client.QueryRowContext(ctx,
		`SELECT MAX(created_at) FROM cp_action_tokens WHERE account_id = ? AND action = ?`,
		accountID, action,
	).Scan(&lastIssued)
	if err != nil {
		return time.Time{}, fmt.Errorf("infra.TokenRepository.MostRecentIssuedAt: %w", err)
	}
	if !lastIssued.Valid {
		return time.Time{}, nil
	}
	return lastIssued.Time, nil
}
