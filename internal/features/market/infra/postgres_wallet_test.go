//go:build integration

package infra

import (
	"context"
	"errors"
	"testing"

	"github.com/hayakawakaki/go-racp/internal/features/market/domain"
	"github.com/hayakawakaki/go-racp/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ domain.WalletRepository = (*WalletRepository)(nil)

func setupWalletRepo(t *testing.T) (*WalletRepository, *pgxpool.Pool) {
	t.Helper()
	pool := testutil.OpenPostgres(t, "DB_CP_TEST_URL")
	testutil.LockPostgres(t, pool, testutil.CurrencyLockKey)
	testutil.TruncatePostgres(t, pool, "cp_currency")
	testutil.TruncatePostgres(t, pool, "cp_currency_hold")

	return NewWalletRepository(pool), pool
}

func seedWallet(t *testing.T, pool *pgxpool.Pool, accountID int, zeny int64, cashpoint int) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO cp_currency (account_id, zeny, cashpoint) VALUES ($1, $2, $3)`,
		accountID, zeny, cashpoint); err != nil {
		t.Fatalf("seed wallet: %v", err)
	}
}

func walletBalance(t *testing.T, pool *pgxpool.Pool, accountID int) (zeny int64, cashpoint int) {
	t.Helper()
	if err := pool.QueryRow(context.Background(),
		`SELECT zeny, cashpoint FROM cp_currency WHERE account_id = $1`, accountID,
	).Scan(&zeny, &cashpoint); err != nil {
		t.Fatalf("read balance: %v", err)
	}

	return zeny, cashpoint
}

func holdCount(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM cp_currency_hold`).Scan(&count); err != nil {
		t.Fatalf("count holds: %v", err)
	}

	return count
}

func TestWalletRepository_Hold(t *testing.T) {
	repo, pool := setupWalletRepo(t)
	ctx := context.Background()
	seedWallet(t, pool, 1, 1000, 0)

	holdID, err := repo.Hold(ctx, 1, 600, 0)
	if err != nil {
		t.Fatalf("Hold: %v", err)
	}
	if holdID == 0 {
		t.Error("holdID = 0, want a non zero id")
	}
	if zeny, _ := walletBalance(t, pool, 1); zeny != 400 {
		t.Errorf("balance after hold = %d, want 400", zeny)
	}
	if holdCount(t, pool) != 1 {
		t.Errorf("hold rows = %d, want 1", holdCount(t, pool))
	}

	if _, err := repo.Hold(ctx, 1, 10000, 0); !errors.Is(err, domain.ErrInsufficientFunds) {
		t.Errorf("Hold over balance err = %v, want ErrInsufficientFunds", err)
	}
	if _, err := repo.Hold(ctx, 1, -1, 0); !errors.Is(err, domain.ErrInvalidAmount) {
		t.Errorf("Hold negative err = %v, want ErrInvalidAmount", err)
	}
}

func TestWalletRepository_Release(t *testing.T) {
	repo, pool := setupWalletRepo(t)
	ctx := context.Background()
	seedWallet(t, pool, 1, 1000, 0)

	holdID, err := repo.Hold(ctx, 1, 600, 0)
	if err != nil {
		t.Fatalf("Hold: %v", err)
	}

	if err := repo.Release(ctx, holdID); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if zeny, _ := walletBalance(t, pool, 1); zeny != 1000 {
		t.Errorf("balance after release = %d, want 1000 (fully restored)", zeny)
	}
	if holdCount(t, pool) != 0 {
		t.Errorf("hold rows after release = %d, want 0", holdCount(t, pool))
	}

	if err := repo.Release(ctx, holdID); !errors.Is(err, domain.ErrHoldNotFound) {
		t.Errorf("second Release err = %v, want ErrHoldNotFound", err)
	}
}

func TestWalletRepository_SettleHold(t *testing.T) {
	repo, pool := setupWalletRepo(t)
	ctx := context.Background()
	seedWallet(t, pool, 1, 1000, 0)

	holdID, err := repo.Hold(ctx, 1, 600, 0)
	if err != nil {
		t.Fatalf("Hold: %v", err)
	}

	if err := repo.SettleHold(ctx, holdID, 2, 500, 0); err != nil {
		t.Fatalf("SettleHold: %v", err)
	}
	if zeny, _ := walletBalance(t, pool, 2); zeny != 500 {
		t.Errorf("payee balance = %d, want 500 (net, 100 fee burned)", zeny)
	}
	if zeny, _ := walletBalance(t, pool, 1); zeny != 400 {
		t.Errorf("payer balance = %d, want 400 (debited at hold)", zeny)
	}
	if holdCount(t, pool) != 0 {
		t.Errorf("hold rows after settle = %d, want 0", holdCount(t, pool))
	}

	if err := repo.SettleHold(ctx, holdID, 2, 500, 0); !errors.Is(err, domain.ErrHoldNotFound) {
		t.Errorf("double settle err = %v, want ErrHoldNotFound", err)
	}
}

func TestWalletRepository_Charge(t *testing.T) {
	repo, pool := setupWalletRepo(t)
	ctx := context.Background()
	seedWallet(t, pool, 1, 1000, 0)

	if err := repo.Charge(ctx, 1, 2, 600, 0, 588, 0); err != nil {
		t.Fatalf("Charge: %v", err)
	}
	payerZeny, _ := walletBalance(t, pool, 1)
	payeeZeny, _ := walletBalance(t, pool, 2)
	if payerZeny != 400 {
		t.Errorf("payer balance = %d, want 400", payerZeny)
	}
	if payeeZeny != 588 {
		t.Errorf("payee balance = %d, want 588 (net, 12 fee burned)", payeeZeny)
	}

	if err := repo.Charge(ctx, 1, 1, 100, 0, 100, 0); !errors.Is(err, domain.ErrSelfTrade) {
		t.Errorf("self trade err = %v, want ErrSelfTrade", err)
	}
	if err := repo.Charge(ctx, 1, 2, 100000, 0, 98000, 0); !errors.Is(err, domain.ErrInsufficientFunds) {
		t.Errorf("over balance charge err = %v, want ErrInsufficientFunds", err)
	}
}

func TestWalletRepository_Burn(t *testing.T) {
	repo, pool := setupWalletRepo(t)
	ctx := context.Background()
	seedWallet(t, pool, 1, 1000, 0)

	if err := repo.Burn(ctx, 1, 200, 0); err != nil {
		t.Fatalf("Burn: %v", err)
	}
	if zeny, _ := walletBalance(t, pool, 1); zeny != 800 {
		t.Errorf("balance after burn = %d, want 800", zeny)
	}

	if err := repo.Burn(ctx, 1, 100000, 0); !errors.Is(err, domain.ErrInsufficientFunds) {
		t.Errorf("over balance burn err = %v, want ErrInsufficientFunds", err)
	}
}

func TestWalletRepository_SettleHoldPartial(t *testing.T) {
	repo, pool := setupWalletRepo(t)
	ctx := context.Background()
	seedWallet(t, pool, 1, 1000, 0)

	holdID, err := repo.Hold(ctx, 1, 600, 0)
	if err != nil {
		t.Fatalf("Hold: %v", err)
	}

	if err := repo.SettleHoldPartial(ctx, holdID, 2, 400, 0, 392, 0); err != nil {
		t.Fatalf("SettleHoldPartial: %v", err)
	}
	if zeny, _ := walletBalance(t, pool, 2); zeny != 392 {
		t.Errorf("payee balance = %d, want 392 (net of 400 gross)", zeny)
	}
	if holdCount(t, pool) != 1 {
		t.Errorf("hold rows after partial settle = %d, want 1 (200 remains)", holdCount(t, pool))
	}

	if err := repo.SettleHoldPartial(ctx, holdID, 2, 200, 0, 196, 0); err != nil {
		t.Fatalf("second SettleHoldPartial: %v", err)
	}
	if zeny, _ := walletBalance(t, pool, 2); zeny != 588 {
		t.Errorf("payee balance = %d, want 588 (392 + 196)", zeny)
	}
	if holdCount(t, pool) != 0 {
		t.Errorf("hold rows after draining = %d, want 0", holdCount(t, pool))
	}
}
