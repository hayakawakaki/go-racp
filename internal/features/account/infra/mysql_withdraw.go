package infra

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
)

type WithdrawQueue struct {
	Client *sql.DB
}

func NewWithdrawQueue(client *sql.DB) *WithdrawQueue {
	return &WithdrawQueue{Client: client}
}

func (r *WithdrawQueue) Insert(ctx context.Context, id int64, accountID int, zeny int64, points int) error {
	if _, err := r.Client.ExecContext(ctx,
		"INSERT INTO cp_withdraw (id, account_id, zeny, points) VALUES (?, ?, ?, ?) ON DUPLICATE KEY UPDATE id = id",
		id, accountID, zeny, points,
	); err != nil {
		return fmt.Errorf("infra.WithdrawQueue.Insert: %w", err)
	}

	return nil
}

func (r *WithdrawQueue) Delivered(ctx context.Context, limit int) ([]domain.DeliveredWithdraw, error) {
	rows, err := r.Client.QueryContext(ctx,
		"SELECT id, delivered_at, zeny, points FROM cp_withdraw WHERE delivered_at > 0 ORDER BY id LIMIT ?", limit,
	)
	if err != nil {
		return nil, fmt.Errorf("infra.WithdrawQueue.Delivered: %w", err)
	}

	return collectRows(rows, func(rows *sql.Rows) (domain.DeliveredWithdraw, error) {
		var row domain.DeliveredWithdraw
		if err := rows.Scan(&row.ID, &row.DeliveredAt, &row.Zeny, &row.Points); err != nil {
			return row, fmt.Errorf("infra.WithdrawQueue.Delivered scan: %w", err)
		}
		return row, nil
	})
}

func (r *WithdrawQueue) ResetDelivered(ctx context.Context, id int64) error {
	if _, err := r.Client.ExecContext(ctx, "UPDATE cp_withdraw SET delivered_at = 0 WHERE id = ?", id); err != nil {
		return fmt.Errorf("infra.WithdrawQueue.ResetDelivered: %w", err)
	}

	return nil
}

func (r *WithdrawQueue) Delete(ctx context.Context, id int64) error {
	if _, err := r.Client.ExecContext(ctx, "DELETE FROM cp_withdraw WHERE id = ?", id); err != nil {
		return fmt.Errorf("infra.WithdrawQueue.Delete: %w", err)
	}

	return nil
}
