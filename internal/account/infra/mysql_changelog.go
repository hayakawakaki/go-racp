package infra

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/hayakawakaki/go-racp/internal/account/domain"
)

type ChangeLogRepository struct {
	Client *sql.DB
}

func NewChangeLogRepository(client *sql.DB) *ChangeLogRepository {
	return &ChangeLogRepository{Client: client}
}

func (r *ChangeLogRepository) Record(ctx context.Context, accountID int, changeType domain.ChangeType, at time.Time) error {
	_, err := r.Client.ExecContext(ctx,
		`INSERT INTO cp_account_record (account_id, change_type, changed_at)
		 VALUES (?, ?, ?)
		 ON DUPLICATE KEY UPDATE changed_at = VALUES(changed_at)`,
		accountID, changeType, at,
	)
	if err != nil {
		return fmt.Errorf("infra.ChangeLogRepository.Record: %w", err)
	}
	return nil
}

func (r *ChangeLogRepository) MostRecent(ctx context.Context, accountID int, changeType domain.ChangeType) (time.Time, error) {
	var changedAt time.Time
	err := r.Client.QueryRowContext(ctx,
		`SELECT changed_at FROM cp_account_record WHERE account_id = ? AND change_type = ?`,
		accountID, changeType,
	).Scan(&changedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("infra.ChangeLogRepository.MostRecent: %w", err)
	}
	return changedAt, nil
}
