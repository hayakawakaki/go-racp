package domain

import (
	"context"
	"errors"
	"math"
	"time"
)

var (
	ErrInsufficientBalance = errors.New("currency: insufficient balance")
	ErrInvalidAmount       = errors.New("currency: invalid amount")
	ErrAmountOverflow      = errors.New("currency: amount overflow")
	ErrWithdrawLocked      = errors.New("currency: withdraw is on cooldown")
	ErrDepositLocked       = errors.New("currency: deposit is on cooldown")
)

type Balance struct {
	AccountID int
	Zeny      int64
	Cashpoint int
}

func AddZeny(current, delta int64) (int64, error) {
	if delta < 0 {
		return 0, ErrInvalidAmount
	}
	if current > math.MaxInt64-delta {
		return 0, ErrAmountOverflow
	}

	return current + delta, nil
}

func AddCashpoint(current, delta int) (int, error) {
	if delta < 0 {
		return 0, ErrInvalidAmount
	}
	if current > math.MaxInt32-delta {
		return 0, ErrAmountOverflow
	}

	return current + delta, nil
}

func SubZeny(current, delta int64) (int64, error) {
	if delta < 0 {
		return 0, ErrInvalidAmount
	}
	if delta > current {
		return 0, ErrInsufficientBalance
	}

	return current - delta, nil
}

func SubCashpoint(current, delta int) (int, error) {
	if delta < 0 {
		return 0, ErrInvalidAmount
	}
	if delta > current {
		return 0, ErrInsufficientBalance
	}

	return current - delta, nil
}

func AddBalance(currentZeny, deltaZeny int64, currentCashpoint, deltaCashpoint int) (newZeny int64, newCashpoint int, err error) {
	newZeny, err = AddZeny(currentZeny, deltaZeny)
	if err != nil {
		return 0, 0, err
	}

	newCashpoint, err = AddCashpoint(currentCashpoint, deltaCashpoint)
	if err != nil {
		return 0, 0, err
	}

	return newZeny, newCashpoint, nil
}

func SubBalance(currentZeny, deltaZeny int64, currentCashpoint, deltaCashpoint int) (newZeny int64, newCashpoint int, err error) {
	newZeny, err = SubZeny(currentZeny, deltaZeny)
	if err != nil {
		return 0, 0, err
	}

	newCashpoint, err = SubCashpoint(currentCashpoint, deltaCashpoint)
	if err != nil {
		return 0, 0, err
	}

	return newZeny, newCashpoint, nil
}

type DepositRow struct {
	ID        int64
	AccountID int
	Zeny      int64
	Points    int
}

type WithdrawRequest struct {
	ID        int64
	AccountID int
	Zeny      int64
	Cashpoint int
}

type CurrencyRepository interface {
	Balance(ctx context.Context, accountID int) (Balance, error)
	CreditDeposit(ctx context.Context, depositID int64, accountID int, zeny int64, cashpoint int, lockUntil, now time.Time) (bool, error)
	RequestWithdraw(ctx context.Context, accountID int, zeny int64, cashpoint int, lockUntil, now time.Time) (int64, error)
	PendingWithdraws(ctx context.Context, limit int) ([]WithdrawRequest, error)
	MarkWithdrawSent(ctx context.Context, id int64, now time.Time) error
	RecentWithdraws(ctx context.Context, accountID, limit int) ([]WithdrawRequest, error)
}

type DepositQueue interface {
	Batch(ctx context.Context, limit int) ([]DepositRow, error)
	Delete(ctx context.Context, id int64) error
}

type WithdrawQueue interface {
	Insert(ctx context.Context, id int64, accountID int, zeny int64, points int) error
}
