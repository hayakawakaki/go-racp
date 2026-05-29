//go:build integration

package infra

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
	"github.com/hayakawakaki/go-racp/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ domain.CurrencyRepository = (*CurrencyRepository)(nil)

func setupCurrencyRepo(t *testing.T) (*CurrencyRepository, *pgxpool.Pool) {
	t.Helper()
	pool := testutil.OpenPostgres(t, "DB_CP_TEST_URL")
	testutil.TruncatePostgres(t, pool, "cp_currency")
	testutil.TruncatePostgres(t, pool, "cp_deposit_processed")
	testutil.TruncatePostgres(t, pool, "cp_withdraw_requests")

	return NewCurrencyRepository(pool), pool
}

func seedBalance(t *testing.T, pool *pgxpool.Pool, accountID int, zeny int64, cashpoint int, lockedUntil *time.Time) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO cp_currency (account_id, zeny, cashpoint, locked_until) VALUES ($1, $2, $3, $4)`,
		accountID, zeny, cashpoint, lockedUntil); err != nil {
		t.Fatalf("seed balance: %v", err)
	}
}

func TestCurrencyRepository_Totals(t *testing.T) {
	repo, pool := setupCurrencyRepo(t)
	ctx := context.Background()
	seedBalance(t, pool, 1, 5000, 250, nil)
	seedBalance(t, pool, 2, 3000, 100, nil)

	totals, err := repo.Totals(ctx)
	if err != nil {
		t.Fatalf("Totals: %v", err)
	}
	if totals.Zeny != 8000 || totals.Cashpoint != 350 {
		t.Errorf("Totals = %+v, want {Zeny:8000 Cashpoint:350}", totals)
	}
}

func TestCurrencyRepository_ListDeposits(t *testing.T) {
	repo, _ := setupCurrencyRepo(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	for index := 1; index <= 3; index++ {
		if _, err := repo.CreditDeposit(ctx, int64(index), index, int64(index*100), index, now, now); err != nil {
			t.Fatalf("CreditDeposit %d: %v", index, err)
		}
	}

	rows, total, err := repo.ListDeposits(ctx, 2, 0)
	if err != nil {
		t.Fatalf("ListDeposits: %v", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2 (limit)", len(rows))
	}
	if rows[0].DepositID != 3 {
		t.Errorf("first deposit id = %d, want 3 (newest first)", rows[0].DepositID)
	}
}

func TestCurrencyRepository_ListWithdraws(t *testing.T) {
	repo, pool := setupCurrencyRepo(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	seedBalance(t, pool, 1, 100000, 0, nil)
	for index := 0; index < 3; index++ {
		if _, err := repo.RequestWithdraw(ctx, 1, 100, 0, now, now); err != nil {
			t.Fatalf("RequestWithdraw: %v", err)
		}
	}

	rows, total, err := repo.ListWithdraws(ctx, 2, 0)
	if err != nil {
		t.Fatalf("ListWithdraws: %v", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if len(rows) != 2 || rows[0].ID < rows[1].ID {
		t.Errorf("rows not newest-first: %+v", rows)
	}
	if rows[0].Status != 1 {
		t.Errorf("status = %d, want 1 (pending)", rows[0].Status)
	}
}

func TestCurrencyRepository_Balance_NoRowIsZero(t *testing.T) {
	repo, _ := setupCurrencyRepo(t)

	balance, err := repo.Balance(context.Background(), 999)
	if err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if balance.AccountID != 999 || balance.Zeny != 0 || balance.Cashpoint != 0 {
		t.Errorf("Balance = %+v, want zero for account 999", balance)
	}
}

func TestCurrencyRepository_CreditDeposit_CreditsBalance(t *testing.T) {
	repo, _ := setupCurrencyRepo(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	credited, err := repo.CreditDeposit(ctx, 1, 42, 5000, 250, now.Add(time.Minute), now)
	if err != nil {
		t.Fatalf("CreditDeposit: %v", err)
	}
	if !credited {
		t.Fatalf("CreditDeposit credited = false, want true")
	}

	balance, err := repo.Balance(ctx, 42)
	if err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if balance.Zeny != 5000 || balance.Cashpoint != 250 {
		t.Errorf("Balance = %+v, want {Zeny:5000 Cashpoint:250}", balance)
	}
}

func TestCurrencyRepository_CreditDeposit_IsIdempotent(t *testing.T) {
	repo, _ := setupCurrencyRepo(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	if _, err := repo.CreditDeposit(ctx, 1, 42, 5000, 250, now, now); err != nil {
		t.Fatalf("first CreditDeposit: %v", err)
	}

	credited, err := repo.CreditDeposit(ctx, 1, 42, 5000, 250, now, now)
	if err != nil {
		t.Fatalf("second CreditDeposit: %v", err)
	}
	if credited {
		t.Errorf("second CreditDeposit credited = true, want false for duplicate deposit id")
	}

	balance, err := repo.Balance(ctx, 42)
	if err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if balance.Zeny != 5000 || balance.Cashpoint != 250 {
		t.Errorf("Balance = %+v, want unchanged {Zeny:5000 Cashpoint:250}", balance)
	}
}

func TestCurrencyRepository_RequestWithdraw_DebitsAndEnqueues(t *testing.T) {
	repo, pool := setupCurrencyRepo(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	seedBalance(t, pool, 42, 5000, 250, nil)

	requestID, err := repo.RequestWithdraw(ctx, 42, 1000, 50, now.Add(time.Hour), now)
	if err != nil {
		t.Fatalf("RequestWithdraw: %v", err)
	}
	if requestID <= 0 {
		t.Fatalf("RequestWithdraw id = %d, want > 0", requestID)
	}

	balance, err := repo.Balance(ctx, 42)
	if err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if balance.Zeny != 4000 || balance.Cashpoint != 200 {
		t.Errorf("Balance = %+v, want {Zeny:4000 Cashpoint:200}", balance)
	}

	pending, err := repo.PendingWithdraws(ctx, 10)
	if err != nil {
		t.Fatalf("PendingWithdraws: %v", err)
	}
	if len(pending) != 1 || pending[0].ID != requestID || pending[0].Zeny != 1000 || pending[0].Cashpoint != 50 {
		t.Errorf("PendingWithdraws = %+v, want one row matching the request", pending)
	}
}

func TestCurrencyRepository_RequestWithdraw_Insufficient(t *testing.T) {
	repo, pool := setupCurrencyRepo(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	seedBalance(t, pool, 42, 100, 0, nil)

	_, err := repo.RequestWithdraw(ctx, 42, 1000, 0, now.Add(time.Hour), now)
	if !errors.Is(err, domain.ErrInsufficientBalance) {
		t.Fatalf("RequestWithdraw err = %v, want ErrInsufficientBalance", err)
	}

	balance, err := repo.Balance(ctx, 42)
	if err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if balance.Zeny != 100 {
		t.Errorf("Balance Zeny = %d, want unchanged 100", balance.Zeny)
	}

	pending, err := repo.PendingWithdraws(ctx, 10)
	if err != nil {
		t.Fatalf("PendingWithdraws: %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("PendingWithdraws = %+v, want none after a failed withdraw", pending)
	}
}

func TestCurrencyRepository_SharedCooldown_DepositBlocksWithdraw(t *testing.T) {
	repo, _ := setupCurrencyRepo(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	if _, err := repo.CreditDeposit(ctx, 1, 42, 5000, 250, now.Add(time.Hour), now); err != nil {
		t.Fatalf("CreditDeposit: %v", err)
	}

	_, err := repo.RequestWithdraw(ctx, 42, 100, 0, now.Add(time.Hour), now)
	if !errors.Is(err, domain.ErrWithdrawLocked) {
		t.Errorf("RequestWithdraw err = %v, want ErrWithdrawLocked while deposit cooldown is active", err)
	}
}

func TestCurrencyRepository_SharedCooldown_WithdrawBlocksDeposit(t *testing.T) {
	repo, pool := setupCurrencyRepo(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	seedBalance(t, pool, 42, 5000, 250, nil)

	if _, err := repo.RequestWithdraw(ctx, 42, 100, 0, now.Add(time.Hour), now); err != nil {
		t.Fatalf("RequestWithdraw: %v", err)
	}

	_, err := repo.CreditDeposit(ctx, 1, 42, 1000, 0, now.Add(time.Hour), now)
	if !errors.Is(err, domain.ErrDepositLocked) {
		t.Errorf("CreditDeposit err = %v, want ErrDepositLocked while withdraw cooldown is active", err)
	}
}

func TestCurrencyRepository_WithdrawLifecycle(t *testing.T) {
	repo, pool := setupCurrencyRepo(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	seedBalance(t, pool, 42, 5000, 250, nil)

	requestID, err := repo.RequestWithdraw(ctx, 42, 1000, 50, now, now)
	if err != nil {
		t.Fatalf("RequestWithdraw: %v", err)
	}

	if err := repo.MarkWithdrawSent(ctx, requestID, now); err != nil {
		t.Fatalf("MarkWithdrawSent: %v", err)
	}
	pending, err := repo.PendingWithdraws(ctx, 10)
	if err != nil {
		t.Fatalf("PendingWithdraws: %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("PendingWithdraws after MarkWithdrawSent = %+v, want none", pending)
	}

	if err := repo.MarkWithdrawPending(ctx, requestID); err != nil {
		t.Fatalf("MarkWithdrawPending: %v", err)
	}
	pending, err = repo.PendingWithdraws(ctx, 10)
	if err != nil {
		t.Fatalf("PendingWithdraws: %v", err)
	}
	if len(pending) != 1 || pending[0].ID != requestID {
		t.Errorf("PendingWithdraws after revert = %+v, want the request back as pending", pending)
	}

	recent, err := repo.RecentWithdraws(ctx, 42, 5)
	if err != nil {
		t.Fatalf("RecentWithdraws: %v", err)
	}
	if len(recent) != 1 || recent[0].ID != requestID {
		t.Errorf("RecentWithdraws = %+v, want the request", recent)
	}
}

func TestCurrencyRepository_RequestWithdraw_ConcurrentNoDoubleSpend(t *testing.T) {
	repo, pool := setupCurrencyRepo(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	seedBalance(t, pool, 42, 100, 0, nil)

	var wg sync.WaitGroup
	results := make([]error, 2)
	for index := range results {
		wg.Add(1)
		go func(slot int) {
			defer wg.Done()
			_, err := repo.RequestWithdraw(ctx, 42, 100, 0, now, now)
			results[slot] = err
		}(index)
	}
	wg.Wait()

	successes, insufficient := 0, 0
	for _, err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, domain.ErrInsufficientBalance):
			insufficient++
		default:
			t.Fatalf("unexpected error from concurrent withdraw: %v", err)
		}
	}
	if successes != 1 || insufficient != 1 {
		t.Errorf("successes=%d insufficient=%d, want exactly one of each", successes, insufficient)
	}

	balance, err := repo.Balance(ctx, 42)
	if err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if balance.Zeny != 0 {
		t.Errorf("final Zeny = %d, want 0 (FOR UPDATE must prevent double spend)", balance.Zeny)
	}
}
