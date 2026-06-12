package infra

import (
	"context"
	"errors"
	"fmt"
	"math"

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
	return zeny >= 0 && zeny <= domain.MaxTransferZeny && cashpoint >= 0 && cashpoint <= math.MaxInt32
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

func lockBalance(ctx context.Context, q domain.DBTX, accountID int) (zeny int64, cashpoint int, err error) {
	if _, err = q.Exec(ctx,
		`INSERT INTO cp_currency (account_id) VALUES ($1) ON CONFLICT (account_id) DO NOTHING`, accountID,
	); err != nil {
		return 0, 0, fmt.Errorf("infra.lockBalance ensure: %w", err)
	}

	if err = q.QueryRow(ctx,
		`SELECT zeny, cashpoint FROM cp_currency WHERE account_id = $1 FOR UPDATE`, accountID,
	).Scan(&zeny, &cashpoint); err != nil {
		return 0, 0, fmt.Errorf("infra.lockBalance read: %w", err)
	}

	return zeny, cashpoint, nil
}

func applyDebit(ctx context.Context, q domain.DBTX, accountID int, currentZeny int64, currentCashpoint int, zeny int64, cashpoint int) error {
	if currentZeny < zeny || currentCashpoint < cashpoint {
		return domain.ErrInsufficientFunds
	}

	if _, err := q.Exec(ctx,
		`UPDATE cp_currency SET zeny = zeny - $1, cashpoint = cashpoint - $2 WHERE account_id = $3`,
		zeny, cashpoint, accountID,
	); err != nil {
		return fmt.Errorf("infra.applyDebit: %w", err)
	}

	return nil
}

func applyCredit(ctx context.Context, q domain.DBTX, accountID int, currentZeny int64, currentCashpoint int, zeny int64, cashpoint int) error {
	newZeny, newCashpoint, err := accdomain.AddBalance(currentZeny, zeny, currentCashpoint, cashpoint)
	if err != nil {
		return fmt.Errorf("infra.applyCredit: %w", err)
	}

	if _, err := q.Exec(ctx,
		`UPDATE cp_currency SET zeny = $1, cashpoint = $2 WHERE account_id = $3`,
		newZeny, newCashpoint, accountID,
	); err != nil {
		return fmt.Errorf("infra.applyCredit: %w", err)
	}

	return nil
}

func debitBalance(ctx context.Context, q domain.DBTX, accountID int, zeny int64, cashpoint int) error {
	currentZeny, currentCashpoint, err := lockBalance(ctx, q, accountID)
	if err != nil {
		return err
	}

	return applyDebit(ctx, q, accountID, currentZeny, currentCashpoint, zeny, cashpoint)
}

func creditBalance(ctx context.Context, q domain.DBTX, accountID int, zeny int64, cashpoint int) error {
	currentZeny, currentCashpoint, err := lockBalance(ctx, q, accountID)
	if err != nil {
		return err
	}

	return applyCredit(ctx, q, accountID, currentZeny, currentCashpoint, zeny, cashpoint)
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

	if err = debitBalance(ctx, tx, accountID, zeny, cashpoint); err != nil {
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

func readHold(ctx context.Context, q domain.DBTX, holdID int64) (accountID int, zeny int64, cashpoint int, err error) {
	err = q.QueryRow(ctx,
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

func (r *WalletRepository) ReleaseTx(ctx context.Context, q domain.DBTX, holdID int64) error {
	accountID, zeny, cashpoint, err := readHold(ctx, q, holdID)
	if err != nil {
		return err
	}

	if creditErr := creditBalance(ctx, q, accountID, zeny, cashpoint); creditErr != nil {
		return fmt.Errorf("infra.WalletRepository.ReleaseTx credit: %w", creditErr)
	}

	if _, execErr := q.Exec(ctx, `DELETE FROM cp_currency_hold WHERE id = $1`, holdID); execErr != nil {
		return fmt.Errorf("infra.WalletRepository.ReleaseTx clear: %w", execErr)
	}

	return nil
}

func (r *WalletRepository) Release(ctx context.Context, holdID int64) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("infra.WalletRepository.Release begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if releaseErr := r.ReleaseTx(ctx, tx, holdID); releaseErr != nil {
		return releaseErr
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

func validatePartialSettle(grossZeny int64, grossCashpoint int, payeeZeny int64, payeeCashpoint int) error {
	if !amountsValid(grossZeny, grossCashpoint) || !amountsValid(payeeZeny, payeeCashpoint) {
		return domain.ErrInvalidAmount
	}
	if payeeZeny > grossZeny || payeeCashpoint > grossCashpoint {
		return domain.ErrInvalidSettlement
	}

	return nil
}

func reduceHold(ctx context.Context, q domain.DBTX, holdID, remainingZeny int64, remainingCashpoint int) error {
	if remainingZeny == 0 && remainingCashpoint == 0 {
		if _, err := q.Exec(ctx, `DELETE FROM cp_currency_hold WHERE id = $1`, holdID); err != nil {
			return fmt.Errorf("infra.reduceHold delete: %w", err)
		}

		return nil
	}

	if _, err := q.Exec(ctx,
		`UPDATE cp_currency_hold SET zeny = $1, cashpoint = $2 WHERE id = $3`,
		remainingZeny, remainingCashpoint, holdID,
	); err != nil {
		return fmt.Errorf("infra.reduceHold update: %w", err)
	}

	return nil
}

func (r *WalletRepository) SettleHoldPartialTx(ctx context.Context, q domain.DBTX, holdID int64, payeeAccountID int, grossZeny int64, grossCashpoint int, payeeZeny int64, payeeCashpoint int) error {
	if err := validatePartialSettle(grossZeny, grossCashpoint, payeeZeny, payeeCashpoint); err != nil {
		return err
	}

	_, heldZeny, heldCashpoint, err := readHold(ctx, q, holdID)
	if err != nil {
		return err
	}
	if grossZeny > heldZeny || grossCashpoint > heldCashpoint {
		return domain.ErrInvalidSettlement
	}

	if creditErr := creditBalance(ctx, q, payeeAccountID, payeeZeny, payeeCashpoint); creditErr != nil {
		return fmt.Errorf("infra.WalletRepository.SettleHoldPartialTx credit: %w", creditErr)
	}

	if reduceErr := reduceHold(ctx, q, holdID, heldZeny-grossZeny, heldCashpoint-grossCashpoint); reduceErr != nil {
		return fmt.Errorf("infra.WalletRepository.SettleHoldPartialTx reduce: %w", reduceErr)
	}

	return nil
}

func (r *WalletRepository) SettleHoldPartial(ctx context.Context, holdID int64, payeeAccountID int, grossZeny int64, grossCashpoint int, payeeZeny int64, payeeCashpoint int) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("infra.WalletRepository.SettleHoldPartial begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if settleErr := r.SettleHoldPartialTx(ctx, tx, holdID, payeeAccountID, grossZeny, grossCashpoint, payeeZeny, payeeCashpoint); settleErr != nil {
		return settleErr
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("infra.WalletRepository.SettleHoldPartial commit: %w", err)
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

func lockPair(ctx context.Context, q domain.DBTX, payerAccountID, payeeAccountID int) (payerZeny int64, payerCashpoint int, payeeZeny int64, payeeCashpoint int, err error) {
	first, second := payerAccountID, payeeAccountID
	if second < first {
		first, second = second, first
	}

	firstZeny, firstCashpoint, err := lockBalance(ctx, q, first)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	secondZeny, secondCashpoint, err := lockBalance(ctx, q, second)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	if payeeAccountID == first {
		return secondZeny, secondCashpoint, firstZeny, firstCashpoint, nil
	}

	return firstZeny, firstCashpoint, secondZeny, secondCashpoint, nil
}

func (r *WalletRepository) ChargeTx(ctx context.Context, q domain.DBTX, payerAccountID, payeeAccountID int, payZeny int64, payCashpoint int, payeeZeny int64, payeeCashpoint int) error {
	if err := validateCharge(payerAccountID, payeeAccountID, payZeny, payCashpoint, payeeZeny, payeeCashpoint); err != nil {
		return err
	}

	payerZeny, payerCashpoint, payeeBalanceZeny, payeeBalanceCashpoint, err := lockPair(ctx, q, payerAccountID, payeeAccountID)
	if err != nil {
		return fmt.Errorf("infra.WalletRepository.ChargeTx lock: %w", err)
	}

	if debitErr := applyDebit(ctx, q, payerAccountID, payerZeny, payerCashpoint, payZeny, payCashpoint); debitErr != nil {
		return fmt.Errorf("infra.WalletRepository.ChargeTx debit: %w", debitErr)
	}

	if creditErr := applyCredit(ctx, q, payeeAccountID, payeeBalanceZeny, payeeBalanceCashpoint, payeeZeny, payeeCashpoint); creditErr != nil {
		return fmt.Errorf("infra.WalletRepository.ChargeTx credit: %w", creditErr)
	}

	return nil
}

func (r *WalletRepository) Charge(ctx context.Context, payerAccountID, payeeAccountID int, payZeny int64, payCashpoint int, payeeZeny int64, payeeCashpoint int) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("infra.WalletRepository.Charge begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if chargeErr := r.ChargeTx(ctx, tx, payerAccountID, payeeAccountID, payZeny, payCashpoint, payeeZeny, payeeCashpoint); chargeErr != nil {
		return chargeErr
	}
	if err := tx.Commit(ctx); err != nil {
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

	if err = debitBalance(ctx, tx, accountID, zeny, cashpoint); err != nil {
		return fmt.Errorf("infra.WalletRepository.Burn debit: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("infra.WalletRepository.Burn commit: %w", err)
	}

	return nil
}
