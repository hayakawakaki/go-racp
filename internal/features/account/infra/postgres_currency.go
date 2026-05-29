package infra

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CurrencyRepository struct {
	Pool *pgxpool.Pool
}

func NewCurrencyRepository(pool *pgxpool.Pool) *CurrencyRepository {
	return &CurrencyRepository{Pool: pool}
}

func (r *CurrencyRepository) Balance(ctx context.Context, accountID int) (domain.Balance, error) {
	row := r.Pool.QueryRow(ctx,
		`SELECT account_id, zeny, cashpoint FROM cp_currency WHERE account_id = $1`,
		accountID,
	)

	var balance domain.Balance
	err := row.Scan(&balance.AccountID, &balance.Zeny, &balance.Cashpoint)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Balance{AccountID: accountID}, nil
	}
	if err != nil {
		return domain.Balance{}, fmt.Errorf("infra.CurrencyRepository.Balance: %w", err)
	}

	return balance, nil
}

func (r *CurrencyRepository) lockCurrency(ctx context.Context, tx pgx.Tx, accountID int) (zeny int64, cashpoint int, lockedUntil *time.Time, err error) {
	if _, err = tx.Exec(ctx,
		`INSERT INTO cp_currency (account_id) VALUES ($1) ON CONFLICT (account_id) DO NOTHING`, accountID,
	); err != nil {
		return 0, 0, nil, fmt.Errorf("infra.CurrencyRepository.lockCurrency ensure: %w", err)
	}

	if err = tx.QueryRow(ctx,
		`SELECT zeny, cashpoint, locked_until FROM cp_currency WHERE account_id = $1 FOR UPDATE`, accountID,
	).Scan(&zeny, &cashpoint, &lockedUntil); err != nil {
		return 0, 0, nil, fmt.Errorf("infra.CurrencyRepository.lockCurrency read: %w", err)
	}

	return zeny, cashpoint, lockedUntil, nil
}

func cooldownActive(until *time.Time, now time.Time) bool {
	return until != nil && until.After(now)
}

func (r *CurrencyRepository) CreditDeposit(ctx context.Context, depositID int64, accountID int, zeny int64, cashpoint int, lockUntil, now time.Time) (bool, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("infra.CurrencyRepository.CreditDeposit begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	currentZeny, currentCashpoint, lockedUntil, err := r.lockCurrency(ctx, tx, accountID)
	if err != nil {
		return false, fmt.Errorf("infra.CurrencyRepository.CreditDeposit lock: %w", err)
	}
	if cooldownActive(lockedUntil, now) {
		return false, domain.ErrDepositLocked
	}

	tag, err := tx.Exec(ctx,
		`INSERT INTO cp_deposit_processed (deposit_id, account_id, zeny, cashpoint, processed_at)
		 VALUES ($1, $2, $3, $4, $5) ON CONFLICT (deposit_id) DO NOTHING`,
		depositID, accountID, zeny, cashpoint, now,
	)
	if err != nil {
		return false, fmt.Errorf("infra.CurrencyRepository.CreditDeposit mark: %w", err)
	}
	if tag.RowsAffected() == 0 {
		if err = tx.Commit(ctx); err != nil {
			return false, fmt.Errorf("infra.CurrencyRepository.CreditDeposit commit: %w", err)
		}
		return false, nil
	}

	newZeny, newCashpoint, err := domain.AddBalance(currentZeny, zeny, currentCashpoint, cashpoint)
	if err != nil {
		return false, fmt.Errorf("infra.CurrencyRepository.CreditDeposit balance: %w", err)
	}

	if _, err = tx.Exec(ctx,
		`UPDATE cp_currency SET zeny = $1, cashpoint = $2, locked_until = $3 WHERE account_id = $4`,
		newZeny, newCashpoint, lockUntil, accountID,
	); err != nil {
		return false, fmt.Errorf("infra.CurrencyRepository.CreditDeposit update: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("infra.CurrencyRepository.CreditDeposit commit: %w", err)
	}

	return true, nil
}

func (r *CurrencyRepository) RequestWithdraw(ctx context.Context, accountID int, zeny int64, cashpoint int, lockUntil, now time.Time) (int64, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("infra.CurrencyRepository.RequestWithdraw begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	currentZeny, currentCashpoint, lockedUntil, err := r.lockCurrency(ctx, tx, accountID)
	if err != nil {
		return 0, fmt.Errorf("infra.CurrencyRepository.RequestWithdraw lock: %w", err)
	}
	if cooldownActive(lockedUntil, now) {
		return 0, domain.ErrWithdrawLocked
	}

	newZeny, newCashpoint, err := domain.SubBalance(currentZeny, zeny, currentCashpoint, cashpoint)
	if err != nil {
		return 0, fmt.Errorf("infra.CurrencyRepository.RequestWithdraw balance: %w", err)
	}

	if _, err = tx.Exec(ctx,
		`UPDATE cp_currency SET zeny = $1, cashpoint = $2, locked_until = $3 WHERE account_id = $4`,
		newZeny, newCashpoint, lockUntil, accountID,
	); err != nil {
		return 0, fmt.Errorf("infra.CurrencyRepository.RequestWithdraw debit: %w", err)
	}

	var requestID int64
	if err = tx.QueryRow(ctx,
		`INSERT INTO cp_withdraw_requests (account_id, zeny, cashpoint, status, created_at)
		 VALUES ($1, $2, $3, 1, $4) RETURNING id`,
		accountID, zeny, cashpoint, now,
	).Scan(&requestID); err != nil {
		return 0, fmt.Errorf("infra.CurrencyRepository.RequestWithdraw enqueue: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("infra.CurrencyRepository.RequestWithdraw commit: %w", err)
	}

	return requestID, nil
}

func (r *CurrencyRepository) PendingWithdraws(ctx context.Context, limit int) ([]domain.WithdrawRequest, error) {
	rows, err := r.Pool.Query(ctx,
		`SELECT id, account_id, zeny, cashpoint FROM cp_withdraw_requests WHERE status = 1 ORDER BY id LIMIT $1`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("infra.CurrencyRepository.PendingWithdraws: %w", err)
	}
	defer rows.Close()

	out := []domain.WithdrawRequest{}
	for rows.Next() {
		var withdrawRequest domain.WithdrawRequest
		if err := rows.Scan(&withdrawRequest.ID, &withdrawRequest.AccountID, &withdrawRequest.Zeny, &withdrawRequest.Cashpoint); err != nil {
			return nil, fmt.Errorf("infra.CurrencyRepository.PendingWithdraws scan: %w", err)
		}
		out = append(out, withdrawRequest)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("infra.CurrencyRepository.PendingWithdraws rows: %w", err)
	}

	return out, nil
}

func (r *CurrencyRepository) MarkWithdrawSent(ctx context.Context, id int64, now time.Time) error {
	if _, err := r.Pool.Exec(ctx,
		`UPDATE cp_withdraw_requests SET status = 2, sent_at = $1 WHERE id = $2 AND status = 1`, now, id,
	); err != nil {
		return fmt.Errorf("infra.CurrencyRepository.MarkWithdrawSent: %w", err)
	}

	return nil
}

func (r *CurrencyRepository) MarkWithdrawPending(ctx context.Context, id int64) error {
	if _, err := r.Pool.Exec(ctx,
		`UPDATE cp_withdraw_requests SET status = 1, sent_at = NULL WHERE id = $1 AND status = 2`, id,
	); err != nil {
		return fmt.Errorf("infra.CurrencyRepository.MarkWithdrawPending: %w", err)
	}

	return nil
}

func (r *CurrencyRepository) RecentWithdraws(ctx context.Context, accountID, limit int) ([]domain.WithdrawRequest, error) {
	rows, err := r.Pool.Query(ctx,
		`SELECT id, account_id, zeny, cashpoint FROM cp_withdraw_requests WHERE account_id = $1 ORDER BY id DESC LIMIT $2`,
		accountID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("infra.CurrencyRepository.RecentWithdraws: %w", err)
	}
	defer rows.Close()

	out := []domain.WithdrawRequest{}
	for rows.Next() {
		var withdrawRequest domain.WithdrawRequest
		if err := rows.Scan(&withdrawRequest.ID, &withdrawRequest.AccountID, &withdrawRequest.Zeny, &withdrawRequest.Cashpoint); err != nil {
			return nil, fmt.Errorf("infra.CurrencyRepository.RecentWithdraws scan: %w", err)
		}
		out = append(out, withdrawRequest)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("infra.CurrencyRepository.RecentWithdraws rows: %w", err)
	}

	return out, nil
}
