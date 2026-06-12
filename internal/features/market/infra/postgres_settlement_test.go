//go:build integration

package infra

import (
	"context"
	"testing"

	"github.com/hayakawakaki/go-racp/internal/features/market/domain"
	"github.com/hayakawakaki/go-racp/internal/testutil"
)

var _ domain.SettlementRepository = (*SettlementRepository)(nil)

func TestSettlementRepository_RoundTrip(t *testing.T) {
	pool := testutil.OpenPostgres(t, "DB_CP_TEST_URL")
	testutil.LockPostgres(t, pool, testutil.CurrencyLockKey)
	testutil.TruncatePostgres(t, pool, "cp_settlement")
	repo := NewSettlementRepository(pool)
	ctx := context.Background()

	legs := []domain.SettlementLeg{
		{ListingID: 100, EscrowRef: 100, RecipientAccountID: 5, Whole: true},
		{ListingID: 100, EscrowRef: 200, RecipientAccountID: 6, DeliverAmount: 5, Whole: false},
	}
	for _, leg := range legs {
		if err := repo.Enqueue(ctx, leg); err != nil {
			t.Fatalf("Enqueue: %v", err)
		}
	}

	pending, err := repo.Pending(ctx, 10)
	if err != nil {
		t.Fatalf("Pending: %v", err)
	}
	if len(pending) != 2 {
		t.Fatalf("pending = %d, want 2", len(pending))
	}

	refs, err := repo.PendingRefs(ctx)
	if err != nil {
		t.Fatalf("PendingRefs: %v", err)
	}
	if len(refs) != 2 {
		t.Errorf("pending refs = %d, want 2 (100 and 200)", len(refs))
	}

	if doneErr := repo.MarkDone(ctx, pending[0].ID); doneErr != nil {
		t.Fatalf("MarkDone: %v", doneErr)
	}

	remaining, err := repo.Pending(ctx, 10)
	if err != nil {
		t.Fatalf("Pending after MarkDone: %v", err)
	}
	if len(remaining) != 1 || remaining[0].ID == pending[0].ID {
		t.Errorf("pending after MarkDone = %d (ids should exclude the done leg), want 1", len(remaining))
	}
}
