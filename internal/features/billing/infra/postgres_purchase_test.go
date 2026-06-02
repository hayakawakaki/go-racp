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
	testutil.LockPostgres(t, pool, testutil.CurrencyLockKey)
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

func seedPendingProvider(t *testing.T, repo *PurchaseRepository, accountID, cashPoints int, provider string, createdAt time.Time) int64 {
	t.Helper()
	id, err := repo.Create(context.Background(), domain.Purchase{
		AccountID:  accountID,
		PackageKey: "starter",
		Provider:   provider,
		Amount:     500,
		Currency:   "USD",
		CashPoints: cashPoints,
		Status:     domain.StatusPending,
		CreatedAt:  createdAt,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	return id
}

func seedCompleted(t *testing.T, repo *PurchaseRepository, pool *pgxpool.Pool, accountID int, amount int64, completedAt time.Time) int64 {
	t.Helper()
	id := seedPending(t, repo, accountID, 100)
	if _, err := pool.Exec(context.Background(),
		`UPDATE cp_purchases SET status = 2, amount = $1, completed_at = $2 WHERE id = $3`,
		amount, completedAt, id,
	); err != nil {
		t.Fatalf("seed completed: %v", err)
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

func TestPurchaseRepository_ListPaidByAccount_ExcludesPendingAndFailed(t *testing.T) {
	repo, _ := setupPurchaseRepo(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	completedID := seedPending(t, repo, 42, 100)
	if _, _, _, err := repo.Complete(ctx, completedID, "pay_done", now); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	disputedID := seedPending(t, repo, 42, 100)
	if _, _, _, err := repo.Complete(ctx, disputedID, "pay_dis", now); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if _, err := repo.MarkDisputed(ctx, disputedID, now); err != nil {
		t.Fatalf("MarkDisputed: %v", err)
	}

	refundedID := seedPending(t, repo, 42, 100)
	if _, _, _, err := repo.Complete(ctx, refundedID, "pay_ref", now); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if _, err := repo.MarkRefunded(ctx, refundedID, now); err != nil {
		t.Fatalf("MarkRefunded: %v", err)
	}

	pendingID := seedPending(t, repo, 42, 100)

	failedID := seedPending(t, repo, 42, 100)
	if _, err := repo.MarkFailed(ctx, failedID, now); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}

	seedPending(t, repo, 99, 100)

	rows, err := repo.ListPaidByAccount(ctx, 42, 50)
	if err != nil {
		t.Fatalf("ListPaidByAccount: %v", err)
	}

	got := make(map[int64]int, len(rows))
	for _, p := range rows {
		got[p.ID] = p.Status
		if p.AccountID != 42 {
			t.Errorf("row %d accountID = %d, want 42", p.ID, p.AccountID)
		}
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3 (completed, disputed, refunded)", len(rows))
	}
	if _, ok := got[pendingID]; ok {
		t.Errorf("pending row %d must be excluded", pendingID)
	}
	if _, ok := got[failedID]; ok {
		t.Errorf("failed row %d must be excluded", failedID)
	}
}

func TestPurchaseRepository_ListFiltered_HonorsEachDimension(t *testing.T) {
	repo, _ := setupPurchaseRepo(t)
	ctx := context.Background()
	base := time.Date(2026, time.May, 15, 12, 0, 0, 0, time.UTC)

	stripeOld := seedPendingProvider(t, repo, 42, 100, "stripe", base)
	stripeNew := seedPendingProvider(t, repo, 42, 100, "stripe", base.AddDate(0, 0, 5))
	fakeRow := seedPendingProvider(t, repo, 7, 100, "fake", base.AddDate(0, 0, 2))
	completedRow := seedPending(t, repo, 42, 100)
	if _, _, _, err := repo.Complete(ctx, completedRow, "pay_c", base); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	collectIDs := func(rows []domain.Purchase) map[int64]struct{} {
		out := make(map[int64]struct{}, len(rows))
		for _, p := range rows {
			out[p.ID] = struct{}{}
		}

		return out
	}

	t.Run("status", func(t *testing.T) {
		rows, total, err := repo.ListFiltered(ctx, domain.PurchaseFilter{Status: domain.StatusCompleted}, 50, 0)
		if err != nil {
			t.Fatalf("ListFiltered: %v", err)
		}
		if total != 1 {
			t.Errorf("total = %d, want 1", total)
		}
		if _, ok := collectIDs(rows)[completedRow]; !ok {
			t.Errorf("completed row %d not returned", completedRow)
		}
	})

	t.Run("account", func(t *testing.T) {
		rows, total, err := repo.ListFiltered(ctx, domain.PurchaseFilter{AccountID: 7}, 50, 0)
		if err != nil {
			t.Fatalf("ListFiltered: %v", err)
		}
		if total != 1 {
			t.Errorf("total = %d, want 1", total)
		}
		if _, ok := collectIDs(rows)[fakeRow]; !ok {
			t.Errorf("account 7 row %d not returned", fakeRow)
		}
	})

	t.Run("provider", func(t *testing.T) {
		rows, total, err := repo.ListFiltered(ctx, domain.PurchaseFilter{Provider: "stripe"}, 50, 0)
		if err != nil {
			t.Fatalf("ListFiltered: %v", err)
		}
		if total != 2 {
			t.Errorf("total = %d, want 2", total)
		}
		ids := collectIDs(rows)
		if _, ok := ids[stripeOld]; !ok {
			t.Errorf("stripe row %d not returned", stripeOld)
		}
		if _, ok := ids[stripeNew]; !ok {
			t.Errorf("stripe row %d not returned", stripeNew)
		}
	})

	t.Run("created range", func(t *testing.T) {
		from := base.AddDate(0, 0, 1)
		to := base.AddDate(0, 0, 4)
		rows, total, err := repo.ListFiltered(ctx, domain.PurchaseFilter{From: &from, To: &to}, 50, 0)
		if err != nil {
			t.Fatalf("ListFiltered: %v", err)
		}
		if total != 1 {
			t.Errorf("total = %d, want 1", total)
		}
		if _, ok := collectIDs(rows)[fakeRow]; !ok {
			t.Errorf("in-range row %d not returned", fakeRow)
		}
	})

	t.Run("total independent of page limit", func(t *testing.T) {
		rows, total, err := repo.ListFiltered(ctx, domain.PurchaseFilter{}, 2, 0)
		if err != nil {
			t.Fatalf("ListFiltered: %v", err)
		}
		if total != 4 {
			t.Errorf("total = %d, want 4 (all rows)", total)
		}
		if len(rows) != 2 {
			t.Errorf("page rows = %d, want 2 (limited)", len(rows))
		}
	})
}

func TestPurchaseRepository_Earnings_SumsCompletedInWindows(t *testing.T) {
	repo, pool := setupPurchaseRepo(t)
	ctx := context.Background()

	dayStart := time.Date(2026, time.May, 27, 0, 0, 0, 0, time.UTC)
	weekStart := time.Date(2026, time.May, 25, 0, 0, 0, 0, time.UTC)
	monthStart := time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC)

	seedCompleted(t, repo, pool, 1, 100, dayStart.Add(2*time.Hour))
	seedCompleted(t, repo, pool, 1, 200, weekStart.Add(12*time.Hour))
	seedCompleted(t, repo, pool, 1, 400, monthStart.Add(36*time.Hour))
	seedCompleted(t, repo, pool, 1, 800, monthStart.AddDate(0, -1, 0))

	pendingRow := seedPending(t, repo, 1, 100)
	if _, err := pool.Exec(ctx,
		`UPDATE cp_purchases SET amount = 9999, completed_at = $1 WHERE id = $2`,
		dayStart.Add(time.Hour), pendingRow,
	); err != nil {
		t.Fatalf("seed pending with completed_at: %v", err)
	}

	summary, err := repo.Earnings(ctx, dayStart, weekStart, monthStart)
	if err != nil {
		t.Fatalf("Earnings: %v", err)
	}
	if summary.Today != 100 {
		t.Errorf("Today = %d, want 100", summary.Today)
	}
	if summary.Week != 300 {
		t.Errorf("Week = %d, want 300", summary.Week)
	}
	if summary.Month != 700 {
		t.Errorf("Month = %d, want 700", summary.Month)
	}
	if summary.AllTime != 1500 {
		t.Errorf("AllTime = %d, want 1500", summary.AllTime)
	}
}
