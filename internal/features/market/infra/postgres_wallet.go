package infra

import (
	"context"
	"errors"
	"fmt"

	accdomain "github.com/hayakawakaki/go-racp/internal/features/account/domain"
	"github.com/hayakawakaki/go-racp/internal/features/market/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type WalletRepository struct {
	Pool *pgxpool.Pool
}

func NewWalletRepository(pool *pgxpool.Pool) *WalletRepository {
	return &WalletRepository{Pool: pool}
}

func amountsValid(zeny int64, cashpoint int) bool {
	return zeny >= 0 && cashpoint >= 0
}

func (r *WalletRepository) Balance(ctx context.Context, accountID int) (domain.Wallet, error) {
	var wallet domain.Wallet
	err := r.Pool.QueryRow(ctx,
		`SELECT zeny, cashpoint FROM cp_currency WHERE account_id = $1`, accountID,
	).Scan(&wallet.Zeny, &wallet.Cashpoint)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Wallet{}, nil
	}
	if err != nil {
		return domain.Wallet{}, fmt.Errorf("infra.WalletRepository.Balance: %w", err)
	}

	return wallet, nil
}

func lockBalance(ctx context.Context, tx pgx.Tx, accountID int) (zeny int64, cashpoint int, err error) {
	if _, err = tx.Exec(ctx,
		`INSERT INTO cp_currency (account_id) VALUES ($1) ON CONFLICT (account_id) DO NOTHING`, accountID,
	); err != nil {
		return 0, 0, fmt.Errorf("infra.lockBalance ensure: %w", err)
	}

	if err = tx.QueryRow(ctx,
		`SELECT zeny, cashpoint FROM cp_currency WHERE account_id = $1 FOR UPDATE`, accountID,
	).Scan(&zeny, &cashpoint); err != nil {
		return 0, 0, fmt.Errorf("infra.lockBalance read: %w", err)
	}

	return zeny, cashpoint, nil
}

func (r *WalletRepository) Hold(ctx context.Context, accountID int, zeny int64, cashpoint int) (int64, error) {
	if !amountsValid(zeny, cashpoint) {
		return 0, domain.ErrInvalidAmount
	}

	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("infra.WalletRepository.Hold begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	currentZeny, currentCashpoint, err := lockBalance(ctx, tx, accountID)
	if err != nil {
		return 0, fmt.Errorf("infra.WalletRepository.Hold lock: %w", err)
	}
	if currentZeny < zeny || currentCashpoint < cashpoint {
		return 0, domain.ErrInsufficientFunds
	}

	if _, err = tx.Exec(ctx,
		`UPDATE cp_currency SET zeny = zeny - $1, cashpoint = cashpoint - $2 WHERE account_id = $3`,
		zeny, cashpoint, accountID,
	); err != nil {
		return 0, fmt.Errorf("infra.WalletRepository.Hold debit: %w", err)
	}

	var holdID int64
	if err = tx.QueryRow(ctx,
		`INSERT INTO cp_currency_hold (account_id, zeny, cashpoint) VALUES ($1, $2, $3) RETURNING id`,
		accountID, zeny, cashpoint,
	).Scan(&holdID); err != nil {
		return 0, fmt.Errorf("infra.WalletRepository.Hold record: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("infra.WalletRepository.Hold commit: %w", err)
	}

	return holdID, nil
}

func readHold(ctx context.Context, tx pgx.Tx, holdID int64) (accountID int, zeny int64, cashpoint int, err error) {
	err = tx.QueryRow(ctx,
		`SELECT account_id, zeny, cashpoint FROM cp_currency_hold WHERE id = $1 FOR UPDATE`, holdID,
	).Scan(&accountID, &zeny, &cashpoint)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, 0, domain.ErrHoldNotFound
	}
	if err != nil {
		return 0, 0, 0, fmt.Errorf("infra.readHold: %w", err)
	}

	return accountID, zeny, cashpoint, nil
}

func creditBalance(ctx context.Context, tx pgx.Tx, accountID int, zeny int64, cashpoint int) error {
	currentZeny, currentCashpoint, err := lockBalance(ctx, tx, accountID)
	if err != nil {
		return err
	}

	newZeny := accdomain.AddZenyCapped(currentZeny, zeny)
	newCashpoint := accdomain.AddCashpointCapped(currentCashpoint, cashpoint)

	if _, err := tx.Exec(ctx,
		`UPDATE cp_currency SET zeny = $1, cashpoint = $2 WHERE account_id = $3`,
		newZeny, newCashpoint, accountID,
	); err != nil {
		return fmt.Errorf("infra.creditBalance: %w", err)
	}

	return nil
}

func (r *WalletRepository) Release(ctx context.Context, holdID int64) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("infra.WalletRepository.Release begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	accountID, zeny, cashpoint, err := readHold(ctx, tx, holdID)
	if err != nil {
		return err
	}

	if err = creditBalance(ctx, tx, accountID, zeny, cashpoint); err != nil {
		return fmt.Errorf("infra.WalletRepository.Release credit: %w", err)
	}

	if _, err = tx.Exec(ctx, `DELETE FROM cp_currency_hold WHERE id = $1`, holdID); err != nil {
		return fmt.Errorf("infra.WalletRepository.Release clear: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("infra.WalletRepository.Release commit: %w", err)
	}

	return nil
}

func (r *WalletRepository) SettleHold(ctx context.Context, holdID int64, payeeAccountID int, payeeZeny int64, payeeCashpoint int) error {
	if !amountsValid(payeeZeny, payeeCashpoint) {
		return domain.ErrInvalidAmount
	}

	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("infra.WalletRepository.SettleHold begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, heldZeny, heldCashpoint, err := readHold(ctx, tx, holdID)
	if err != nil {
		return err
	}
	if payeeZeny > heldZeny || payeeCashpoint > heldCashpoint {
		return domain.ErrInvalidSettlement
	}

	if err = creditBalance(ctx, tx, payeeAccountID, payeeZeny, payeeCashpoint); err != nil {
		return fmt.Errorf("infra.WalletRepository.SettleHold credit: %w", err)
	}

	if _, err = tx.Exec(ctx, `DELETE FROM cp_currency_hold WHERE id = $1`, holdID); err != nil {
		return fmt.Errorf("infra.WalletRepository.SettleHold clear: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("infra.WalletRepository.SettleHold commit: %w", err)
	}

	return nil
}

func validateCharge(payerAccountID, payeeAccountID int, payZeny int64, payCashpoint int, payeeZeny int64, payeeCashpoint int) error {
	if !amountsValid(payZeny, payCashpoint) || !amountsValid(payeeZeny, payeeCashpoint) {
		return domain.ErrInvalidAmount
	}
	if payerAccountID == payeeAccountID {
		return domain.ErrSelfTrade
	}
	if payeeZeny > payZeny || payeeCashpoint > payCashpoint {
		return domain.ErrInvalidSettlement
	}

	return nil
}

func debitLocked(ctx context.Context, tx pgx.Tx, accountID int, zeny int64, cashpoint int) error {
	var currentZeny int64
	var currentCashpoint int
	if err := tx.QueryRow(ctx,
		`SELECT zeny, cashpoint FROM cp_currency WHERE account_id = $1`, accountID,
	).Scan(&currentZeny, &currentCashpoint); err != nil {
		return fmt.Errorf("infra.debitLocked read: %w", err)
	}
	if currentZeny < zeny || currentCashpoint < cashpoint {
		return domain.ErrInsufficientFunds
	}

	if _, err := tx.Exec(ctx,
		`UPDATE cp_currency SET zeny = zeny - $1, cashpoint = cashpoint - $2 WHERE account_id = $3`,
		zeny, cashpoint, accountID,
	); err != nil {
		return fmt.Errorf("infra.debitLocked update: %w", err)
	}

	return nil
}

func (r *WalletRepository) Charge(ctx context.Context, payerAccountID, payeeAccountID int, payZeny int64, payCashpoint int, payeeZeny int64, payeeCashpoint int) error {
	if err := validateCharge(payerAccountID, payeeAccountID, payZeny, payCashpoint, payeeZeny, payeeCashpoint); err != nil {
		return err
	}

	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("infra.WalletRepository.Charge begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	first, second := payerAccountID, payeeAccountID
	if second < first {
		first, second = second, first
	}
	if _, _, err = lockBalance(ctx, tx, first); err != nil {
		return fmt.Errorf("infra.WalletRepository.Charge lock first: %w", err)
	}
	if _, _, err = lockBalance(ctx, tx, second); err != nil {
		return fmt.Errorf("infra.WalletRepository.Charge lock second: %w", err)
	}

	if err = debitLocked(ctx, tx, payerAccountID, payZeny, payCashpoint); err != nil {
		return fmt.Errorf("infra.WalletRepository.Charge debit: %w", err)
	}

	if err = creditBalance(ctx, tx, payeeAccountID, payeeZeny, payeeCashpoint); err != nil {
		return fmt.Errorf("infra.WalletRepository.Charge credit: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("infra.WalletRepository.Charge commit: %w", err)
	}

	return nil
}

func (r *WalletRepository) Burn(ctx context.Context, accountID int, zeny int64, cashpoint int) error {
	if !amountsValid(zeny, cashpoint) {
		return domain.ErrInvalidAmount
	}

	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("infra.WalletRepository.Burn begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	currentZeny, currentCashpoint, err := lockBalance(ctx, tx, accountID)
	if err != nil {
		return fmt.Errorf("infra.WalletRepository.Burn lock: %w", err)
	}
	if currentZeny < zeny || currentCashpoint < cashpoint {
		return domain.ErrInsufficientFunds
	}

	if _, err = tx.Exec(ctx,
		`UPDATE cp_currency SET zeny = zeny - $1, cashpoint = cashpoint - $2 WHERE account_id = $3`,
		zeny, cashpoint, accountID,
	); err != nil {
		return fmt.Errorf("infra.WalletRepository.Burn debit: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("infra.WalletRepository.Burn commit: %w", err)
	}

	return nil
}
