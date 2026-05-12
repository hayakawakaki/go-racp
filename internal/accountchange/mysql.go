package accountchange

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

func (r *MySQLRepository) Record(ctx context.Context, accountID int, changeType Type, at time.Time) error {
	_, err := r.Client.ExecContext(ctx,
		`INSERT INTO cp_account_record (account_id, change_type, changed_at)
		 VALUES (?, ?, ?)
		 ON DUPLICATE KEY UPDATE changed_at = VALUES(changed_at)`,
		accountID, changeType, at,
	)
	if err != nil {
		return fmt.Errorf("accountchange.MySQLRepository.Record: %w", err)
	}
	return nil
}

func (r *MySQLRepository) MostRecent(ctx context.Context, accountID int, changeType Type) (time.Time, error) {
	var changedAt time.Time
	err := r.Client.QueryRowContext(ctx,
		`SELECT changed_at FROM cp_account_record WHERE account_id = ? AND change_type = ?`,
		accountID, changeType,
	).Scan(&changedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("accountchange.MySQLRepository.MostRecent: %w", err)
	}
	return changedAt, nil
}
