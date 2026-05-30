//go:build integration

package infra

import (
	"context"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/billing/domain"
	"github.com/hayakawakaki/go-racp/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

func setupPurchaseRepo(t *testing.T) (*PurchaseRepository, *pgxpool.Pool) {
	t.Helper()
	pool := testutil.OpenPostgres(t, "DB_CP_TEST_URL")
	testutil.TruncatePostgres(t, pool, "cp_purchases")
	testutil.TruncatePostgres(t, pool, "cp_currency")

	return NewPurchaseRepository(pool), pool
}

func seedPending(t *testing.T, repo *PurchaseRepository, accountID, cashPoints int) int64 {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	id, err := repo.Create(context.Background(), domain.Purchase{
		AccountID:  accountID,
		PackageKey: "starter",
		Provider:   "fake",
		Amount:     500,
		Currency:   "USD",
		CashPoints: cashPoints,
		Status:     domain.StatusPending,
		CreatedAt:  now,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	return id
}

func walletCashpoint(t *testing.T, pool *pgxpool.Pool, accountID int) int {
	t.Helper()
	var cashpoint int
	if err := pool.QueryRow(context.Background(),
		`SELECT cashpoint FROM cp_currency WHERE account_id = $1`, accountID,
	).Scan(&cashpoint); err != nil {
		t.Fatalf("read wallet: %v", err)
	}

	return cashpoint
}

func TestPurchaseRepository_Complete_CreditsOnceIdempotent(t *testing.T) {
	repo, pool := setupPurchaseRepo(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	id := seedPending(t, repo, 42, 500)

	credited, accountID, cashPoints, err := repo.Complete(ctx, id, "pay_1", now)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if !credited || accountID != 42 || cashPoints != 500 {
		t.Fatalf("Complete = (%v, %d, %d), want (true, 42, 500)", credited, accountID, cashPoints)
	}
	if got := walletCashpoint(t, pool, 42); got != 500 {
		t.Errorf("wallet cashpoint = %d, want 500", got)
	}

	stored, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if stored.Status != domain.StatusCompleted {
		t.Errorf("status = %d, want %d (completed)", stored.Status, domain.StatusCompleted)
	}

	credited, _, _, err = repo.Complete(ctx, id, "pay_1", now)
	if err != nil {
		t.Fatalf("second Complete: %v", err)
	}
	if credited {
		t.Errorf("second Complete credited = true, want false (idempotent)")
	}
	if got := walletCashpoint(t, pool, 42); got != 500 {
		t.Errorf("wallet cashpoint = %d, want unchanged 500", got)
	}
}

func TestPurchaseRepository_MarkDisputed_TransitionsOnce(t *testing.T) {
	repo, _ := setupPurchaseRepo(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	id := seedPending(t, repo, 7, 100)

	if _, _, _, err := repo.Complete(ctx, id, "pay_1", now); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	transitioned, err := repo.MarkDisputed(ctx, id, now)
	if err != nil {
		t.Fatalf("MarkDisputed: %v", err)
	}
	if !transitioned {
		t.Fatalf("MarkDisputed transitioned = false, want true for a completed row")
	}

	transitioned, err = repo.MarkDisputed(ctx, id, now)
	if err != nil {
		t.Fatalf("second MarkDisputed: %v", err)
	}
	if transitioned {
		t.Errorf("second MarkDisputed transitioned = true, want false (already disputed)")
	}
}

func TestPurchaseRepository_MarkRefunded_GatesOnCompleted(t *testing.T) {
	repo, _ := setupPurchaseRepo(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	id := seedPending(t, repo, 8, 100)

	if _, _, _, err := repo.Complete(ctx, id, "pay_1", now); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	transitioned, err := repo.MarkRefunded(ctx, id, now)
	if err != nil {
		t.Fatalf("MarkRefunded: %v", err)
	}
	if !transitioned {
		t.Fatalf("MarkRefunded transitioned = false, want true for a completed row")
	}

	pending := seedPending(t, repo, 9, 100)
	transitioned, err = repo.MarkRefunded(ctx, pending, now)
	if err != nil {
		t.Fatalf("MarkRefunded on pending: %v", err)
	}
	if transitioned {
		t.Errorf("MarkRefunded on a non-completed row transitioned = true, want false")
	}
}

func TestPurchaseRepository_MarkFailed_GatesOnPending(t *testing.T) {
	repo, _ := setupPurchaseRepo(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	pending := seedPending(t, repo, 10, 100)

	transitioned, err := repo.MarkFailed(ctx, pending, now)
	if err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
	if !transitioned {
		t.Fatalf("MarkFailed transitioned = false, want true for a pending row")
	}

	completed := seedPending(t, repo, 11, 100)
	if _, _, _, err = repo.Complete(ctx, completed, "pay_2", now); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	transitioned, err = repo.MarkFailed(ctx, completed, now)
	if err != nil {
		t.Fatalf("MarkFailed on completed: %v", err)
	}
	if transitioned {
		t.Errorf("MarkFailed on a completed row transitioned = true, want false (cannot undo a credit)")
	}
}
