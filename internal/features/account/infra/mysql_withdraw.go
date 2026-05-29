package infra

import (
	"context"
	"database/sql"
	"fmt"
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
