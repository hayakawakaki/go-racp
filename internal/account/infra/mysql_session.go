package infra

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/hayakawakaki/go-racp/internal/account/domain"
)

type SessionRepository struct {
	Client *sql.DB
}

func NewSessionRepository(client *sql.DB) *SessionRepository {
	return &SessionRepository{Client: client}
}

func (r *SessionRepository) Create(ctx context.Context, s *domain.Session) error {
	_, err := r.Client.ExecContext(ctx,
		`INSERT INTO cp_sessions (token_hash, user_id, expires_at, last_seen_at, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		s.TokenHash[:], s.UserID, s.ExpiresAt, s.LastSeenAt, s.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("infra.SessionRepository.Create: %w", err)
	}
	return nil
}

func (r *SessionRepository) GetByTokenHash(ctx context.Context, hash [32]byte) (*domain.Session, error) {
	var (
		s   domain.Session
		raw []byte
	)
	err := r.Client.QueryRowContext(ctx,
		`SELECT token_hash, user_id, expires_at, last_seen_at, created_at
		 FROM cp_sessions WHERE token_hash = ?`, hash[:],
	).Scan(&raw, &s.UserID, &s.ExpiresAt, &s.LastSeenAt, &s.CreatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("infra.SessionRepository.GetByTokenHash: %w", err)
	}
	if len(raw) != 32 {
		return nil, fmt.Errorf("infra.SessionRepository.GetByTokenHash: token_hash len=%d", len(raw))
	}
	copy(s.TokenHash[:], raw)
	return &s, nil
}

func (r *SessionRepository) Refresh(ctx context.Context, hash [32]byte, lastSeen, expiresAt time.Time) error {
	res, err := r.Client.ExecContext(ctx,
		`UPDATE cp_sessions SET last_seen_at = ?, expires_at = ? WHERE token_hash = ?`,
		lastSeen, expiresAt, hash[:],
	)
	if err != nil {
		return fmt.Errorf("infra.SessionRepository.Refresh: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("infra.SessionRepository.Refresh: %w", err)
	}
	if rows == 0 {
		return domain.ErrSessionNotFound
	}
	return nil
}

func (r *SessionRepository) Delete(ctx context.Context, hash [32]byte) error {
	res, err := r.Client.ExecContext(ctx,
		`DELETE FROM cp_sessions WHERE token_hash = ?`, hash[:],
	)
	if err != nil {
		return fmt.Errorf("infra.SessionRepository.Delete: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("infra.SessionRepository.Delete: %w", err)
	}
	if rows == 0 {
		return domain.ErrSessionNotFound
	}
	return nil
}

func (r *SessionRepository) DeleteByUserID(ctx context.Context, userID int) error {
	if _, err := r.Client.ExecContext(ctx,
		"DELETE FROM cp_sessions WHERE user_id = ?",
		userID,
	); err != nil {
		return fmt.Errorf("infra.SessionRepository.DeleteByUserID: %w", err)
	}
	return nil
}

func (r *SessionRepository) DeleteByUserIDExcept(ctx context.Context, userID int, exceptHash [32]byte) error {
	if _, err := r.Client.ExecContext(ctx,
		"DELETE FROM cp_sessions WHERE user_id = ? AND token_hash <> ?",
		userID, exceptHash[:],
	); err != nil {
		return fmt.Errorf("infra.SessionRepository.DeleteByUserIDExcept: %w", err)
	}
	return nil
}
