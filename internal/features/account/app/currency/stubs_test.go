package currency

import (
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type fakeCurrencyRepo struct {
	balanceFn         func(ctx context.Context, accountID int) (domain.Balance, error)
	creditDepositFn   func(ctx context.Context, depositID int64, accountID int, zeny int64, cashpoint int, lockUntil, now time.Time) (bool, error)
	requestWithdrawFn func(ctx context.Context, accountID int, zeny int64, cashpoint int, lockUntil, now time.Time) (int64, error)
	pendingFn         func(ctx context.Context, limit int) ([]domain.WithdrawRequest, error)
	markSentFn        func(ctx context.Context, id int64, now time.Time) error
	markPendingFn     func(ctx context.Context, id int64) error
	recentFn          func(ctx context.Context, accountID, limit int) ([]domain.WithdrawRequest, error)
}

var _ domain.CurrencyRepository = (*fakeCurrencyRepo)(nil)

func (f *fakeCurrencyRepo) Balance(ctx context.Context, accountID int) (domain.Balance, error) {
	if f.balanceFn != nil {
		return f.balanceFn(ctx, accountID)
	}

	return domain.Balance{}, nil
}

func (f *fakeCurrencyRepo) CreditDeposit(ctx context.Context, depositID int64, accountID int, zeny int64, cashpoint int, lockUntil, now time.Time) (bool, error) {
	if f.creditDepositFn != nil {
		return f.creditDepositFn(ctx, depositID, accountID, zeny, cashpoint, lockUntil, now)
	}

	return true, nil
}

func (f *fakeCurrencyRepo) RequestWithdraw(ctx context.Context, accountID int, zeny int64, cashpoint int, lockUntil, now time.Time) (int64, error) {
	if f.requestWithdrawFn != nil {
		return f.requestWithdrawFn(ctx, accountID, zeny, cashpoint, lockUntil, now)
	}

	return 1, nil
}

func (f *fakeCurrencyRepo) PendingWithdraws(ctx context.Context, limit int) ([]domain.WithdrawRequest, error) {
	if f.pendingFn != nil {
		return f.pendingFn(ctx, limit)
	}

	return nil, nil
}

func (f *fakeCurrencyRepo) MarkWithdrawSent(ctx context.Context, id int64, now time.Time) error {
	if f.markSentFn != nil {
		return f.markSentFn(ctx, id, now)
	}

	return nil
}

func (f *fakeCurrencyRepo) MarkWithdrawPending(ctx context.Context, id int64) error {
	if f.markPendingFn != nil {
		return f.markPendingFn(ctx, id)
	}

	return nil
}

func (f *fakeCurrencyRepo) RecentWithdraws(ctx context.Context, accountID, limit int) ([]domain.WithdrawRequest, error) {
	if f.recentFn != nil {
		return f.recentFn(ctx, accountID, limit)
	}

	return nil, nil
}

type fakeDepositQueue struct {
	batchFn  func(ctx context.Context, limit int) ([]domain.DepositRow, error)
	deleteFn func(ctx context.Context, id int64) error
}

var _ domain.DepositQueue = (*fakeDepositQueue)(nil)

func (f *fakeDepositQueue) Batch(ctx context.Context, limit int) ([]domain.DepositRow, error) {
	if f.batchFn != nil {
		return f.batchFn(ctx, limit)
	}

	return nil, nil
}

func (f *fakeDepositQueue) Delete(ctx context.Context, id int64) error {
	if f.deleteFn != nil {
		return f.deleteFn(ctx, id)
	}

	return nil
}

type fakeWithdrawQueue struct {
	insertFn func(ctx context.Context, id int64, accountID int, zeny int64, points int) error
}

var _ domain.WithdrawQueue = (*fakeWithdrawQueue)(nil)

func (f *fakeWithdrawQueue) Insert(ctx context.Context, id int64, accountID int, zeny int64, points int) error {
	if f.insertFn != nil {
		return f.insertFn(ctx, id, accountID, zeny, points)
	}

	return nil
}
