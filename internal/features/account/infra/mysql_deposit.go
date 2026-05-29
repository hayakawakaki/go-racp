package infra

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
)

type DepositQueue struct {
	Client *sql.DB
}

func NewDepositQueue(client *sql.DB) *DepositQueue {
	return &DepositQueue{Client: client}
}

func (r *DepositQueue) Batch(ctx context.Context, limit int) ([]domain.DepositRow, error) {
	rows, err := r.Client.QueryContext(ctx,
		"SELECT id, account_id, zeny, points FROM cp_deposit ORDER BY id LIMIT ?", limit,
	)
	if err != nil {
		return nil, fmt.Errorf("infra.DepositQueue.Batch: %w", err)
	}

	return collectRows(rows, func(rows *sql.Rows) (domain.DepositRow, error) {
		var depositRow domain.DepositRow
		if err := rows.Scan(&depositRow.ID, &depositRow.AccountID, &depositRow.Zeny, &depositRow.Points); err != nil {
			return depositRow, fmt.Errorf("infra.DepositQueue.Batch scan: %w", err)
		}
		return depositRow, nil
	})
}

func (r *DepositQueue) Delete(ctx context.Context, id int64) error {
	if _, err := r.Client.ExecContext(ctx, "DELETE FROM cp_deposit WHERE id = ?", id); err != nil {
		return fmt.Errorf("infra.DepositQueue.Delete: %w", err)
	}

	return nil
}
