package domain

import "context"

const MaxTransferZeny int64 = 1_000_000_000_000

type Wallet struct {
	Zeny      int64
	Cashpoint int
}

type WalletRepository interface {
	Balance(ctx context.Context, accountID int) (Wallet, error)
	Hold(ctx context.Context, accountID int, zeny int64, cashpoint int) (int64, error)
	Release(ctx context.Context, holdID int64) error
	SettleHold(ctx context.Context, holdID int64, payeeAccountID int, payeeZeny int64, payeeCashpoint int) error
	Charge(ctx context.Context, payerAccountID, payeeAccountID int, payZeny int64, payCashpoint int, payeeZeny int64, payeeCashpoint int) error
	Burn(ctx context.Context, accountID int, zeny int64, cashpoint int) error
}
