//go:build integration

package infra

import (
	"context"
	"testing"

	"github.com/hayakawakaki/go-racp/internal/features/market/domain"
	"github.com/hayakawakaki/go-racp/internal/testutil"
)

func TestEscrowRepository_DeliverPartial(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewEscrowRepository(db, 0, 0)
	ctx := context.Background()

	seller := 9901001
	recipient := 9901002
	ref := int64(9901001)
	cleanupAccounts(t, db, seller, recipient)
	cleanupRefs(t, db, ref)
	cleanupLegs(t, db, 1, 2)

	if _, err := db.Exec("DELETE FROM cp_delivery_applied WHERE leg_id IN (1, 2)"); err != nil {
		t.Fatalf("pre-clean cp_delivery_applied: %v", err)
	}
	if _, err := db.Exec("DELETE FROM cp_storage WHERE account_id IN (?, ?)", seller, recipient); err != nil {
		t.Fatalf("pre-clean cp_storage: %v", err)
	}
	if _, err := db.Exec("DELETE FROM cp_storage_escrow WHERE listing_ref = ?", ref); err != nil {
		t.Fatalf("pre-clean cp_storage_escrow: %v", err)
	}

	setLock(t, db, seller, true)
	setLock(t, db, recipient, true)
	stashID := insertStashRowForTest(t, db, seller, 501, 100)

	if err := repo.MoveToEscrow(ctx, seller, ref, []domain.EscrowMove{{StashItemID: stashID, Amount: 50}}); err != nil {
		t.Fatalf("MoveToEscrow: %v", err)
	}

	recipientAmount := func() int {
		return countRows(t, db, "SELECT CAST(COALESCE(SUM(amount), 0) AS SIGNED) FROM cp_storage WHERE account_id = ? AND nameid = 501", recipient)
	}
	escrowAmount := func() int {
		return countRows(t, db, "SELECT CAST(COALESCE(SUM(amount), 0) AS SIGNED) FROM cp_storage_escrow WHERE listing_ref = ?", ref)
	}

	if err := repo.DeliverPartial(ctx, ref, recipient, 20, 1); err != nil {
		t.Fatalf("DeliverPartial 20: %v", err)
	}
	if got := recipientAmount(); got != 20 {
		t.Errorf("recipient amount = %d, want 20", got)
	}
	if got := escrowAmount(); got != 30 {
		t.Errorf("escrow remaining = %d, want 30", got)
	}

	if err := repo.DeliverPartial(ctx, ref, recipient, 20, 1); err != nil {
		t.Fatalf("DeliverPartial replay: %v", err)
	}
	if got := recipientAmount(); got != 20 {
		t.Errorf("recipient amount after replaying leg 1 = %d, want 20 (idempotent, no double delivery)", got)
	}

	if err := repo.DeliverPartial(ctx, ref, recipient, 30, 2); err != nil {
		t.Fatalf("DeliverPartial 30: %v", err)
	}
	if got := escrowAmount(); got != 0 {
		t.Errorf("escrow remaining after draining = %d, want 0", got)
	}
	if got := countRows(t, db, "SELECT COUNT(*) FROM cp_storage_escrow WHERE listing_ref = ?", ref); got != 0 {
		t.Errorf("escrow rows after draining = %d, want 0 (row deleted)", got)
	}
	if got := recipientAmount(); got != 50 {
		t.Errorf("recipient total = %d, want 50", got)
	}
}
