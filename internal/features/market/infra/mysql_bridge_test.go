//go:build integration

package infra

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/hayakawakaki/go-racp/internal/features/market/domain"
	"github.com/hayakawakaki/go-racp/internal/testutil"
)

type stashRowOption func(*domain.StashItem)

func withUniqueID(uniqueID int64) stashRowOption {
	return func(item *domain.StashItem) { item.UniqueID = uniqueID }
}

func withBound(bound int) stashRowOption {
	return func(item *domain.StashItem) { item.Bound = bound }
}

func insertStashRowForTest(t *testing.T, db *sql.DB, accountID, nameid, amount int, options ...stashRowOption) int64 {
	t.Helper()

	item := domain.StashItem{AccountID: accountID, NameID: nameid, Amount: amount}
	for _, option := range options {
		option(&item)
	}

	res, err := db.Exec(
		`INSERT INTO cp_storage
		 (account_id, nameid, amount, equip, identify, refine, attribute,
		  card0, card1, card2, card3,
		  option_id0, option_val0, option_parm0,
		  option_id1, option_val1, option_parm1,
		  option_id2, option_val2, option_parm2,
		  option_id3, option_val3, option_parm3,
		  option_id4, option_val4, option_parm4,
		  expire_time, bound, unique_id, enchantgrade)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.AccountID, item.NameID, amount, item.Equip, item.Identify, item.Refine, item.Attribute,
		item.Card[0], item.Card[1], item.Card[2], item.Card[3],
		item.OptionID[0], item.OptionVal[0], item.OptionParm[0],
		item.OptionID[1], item.OptionVal[1], item.OptionParm[1],
		item.OptionID[2], item.OptionVal[2], item.OptionParm[2],
		item.OptionID[3], item.OptionVal[3], item.OptionParm[3],
		item.OptionID[4], item.OptionVal[4], item.OptionParm[4],
		item.ExpireTime, item.Bound, item.UniqueID, item.Grade,
	)
	if err != nil {
		t.Fatalf("insert cp_storage account=%d: %v", accountID, err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("cp_storage LastInsertId: %v", err)
	}

	return id
}

func setLock(t *testing.T, db *sql.DB, accountID int, locked bool) {
	t.Helper()

	value := 0
	if locked {
		value = 1
	}

	_, err := db.Exec(
		"INSERT INTO cp_storage_lock (account_id, is_locked) VALUES (?, ?) ON DUPLICATE KEY UPDATE is_locked = VALUES(is_locked)",
		accountID, value,
	)
	if err != nil {
		t.Fatalf("set lock account=%d: %v", accountID, err)
	}
}

func cleanupAccounts(t *testing.T, db *sql.DB, accountIDs ...int) {
	t.Helper()
	t.Cleanup(func() {
		for _, accountID := range accountIDs {
			_, _ = db.Exec("DELETE FROM cp_storage WHERE account_id = ?", accountID)
			_, _ = db.Exec("DELETE FROM cp_storage_escrow WHERE account_id = ?", accountID)
			_, _ = db.Exec("DELETE FROM cp_storage_lock WHERE account_id = ?", accountID)
		}
	})
}

func cleanupRefs(t *testing.T, db *sql.DB, listingRefs ...int64) {
	t.Helper()
	t.Cleanup(func() {
		for _, listingRef := range listingRefs {
			_, _ = db.Exec("DELETE FROM cp_storage_escrow WHERE listing_ref = ?", listingRef)
		}
	})
}

func cleanupLegs(t *testing.T, db *sql.DB, legIDs ...int64) {
	t.Helper()
	t.Cleanup(func() {
		for _, legID := range legIDs {
			_, _ = db.Exec("DELETE FROM cp_delivery_applied WHERE leg_id = ?", legID)
		}
	})
}

func countRows(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()

	var count int
	if err := db.QueryRow(query, args...).Scan(&count); err != nil {
		t.Fatalf("count query %q: %v", query, err)
	}

	return count
}

func TestStashRepository_ListByAccount(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewStashRepository(db)
	accountID := 9900001
	cleanupAccounts(t, db, accountID)

	first := insertStashRowForTest(t, db, accountID, 501, 10)
	insertStashRowForTest(t, db, accountID, 502, 3)

	items, err := repo.ListByAccount(context.Background(), accountID)
	if err != nil {
		t.Fatalf("ListByAccount: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}

	if items[0].ID != first || items[0].NameID != 501 || items[0].Amount != 10 {
		t.Errorf("items[0] = %+v, want id=%d nameid=501 amount=10", items[0], first)
	}
	if items[1].NameID != 502 || items[1].Amount != 3 {
		t.Errorf("items[1] = %+v, want nameid=502 amount=3", items[1])
	}
}

func TestStashRepository_IsLocked(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewStashRepository(db)
	missingAccount := 9900010
	unlockedAccount := 9900011
	lockedAccount := 9900012
	cleanupAccounts(t, db, missingAccount, unlockedAccount, lockedAccount)

	setLock(t, db, unlockedAccount, false)
	setLock(t, db, lockedAccount, true)

	tests := []struct {
		name      string
		accountID int
		want      bool
	}{
		{name: "missing row", accountID: missingAccount, want: false},
		{name: "is_locked 0", accountID: unlockedAccount, want: false},
		{name: "is_locked 1", accountID: lockedAccount, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			locked, err := repo.IsLocked(context.Background(), tt.accountID)
			if err != nil {
				t.Fatalf("IsLocked: %v", err)
			}
			if locked != tt.want {
				t.Errorf("IsLocked = %v, want %v", locked, tt.want)
			}
		})
	}
}

func TestStashRepository_SlotsUsed(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewStashRepository(db)
	accountID := 9900020
	cleanupAccounts(t, db, accountID)

	insertStashRowForTest(t, db, accountID, 501, 10)
	insertStashRowForTest(t, db, accountID, 502, 5)
	insertStashRowForTest(t, db, accountID, 1201, 1)

	used, err := repo.SlotsUsed(context.Background(), accountID)
	if err != nil {
		t.Fatalf("SlotsUsed: %v", err)
	}
	if used != 3 {
		t.Errorf("SlotsUsed = %d, want 3", used)
	}
}

func TestStashRepository_MergeableAmount(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewStashRepository(db)
	accountID := 9900030
	cleanupAccounts(t, db, accountID)

	insertStashRowForTest(t, db, accountID, 501, 12)
	insertStashRowForTest(t, db, accountID, 502, 7, withUniqueID(123456))

	ctx := context.Background()

	plain := domain.StashItem{NameID: 501, Amount: 1}
	amount, found, err := repo.MergeableAmount(ctx, accountID, plain)
	if err != nil {
		t.Fatalf("MergeableAmount plain: %v", err)
	}
	if !found || amount != 12 {
		t.Errorf("plain found=%v amount=%d, want true/12", found, amount)
	}

	carded := domain.StashItem{NameID: 502, Amount: 1, UniqueID: 123456}
	_, found, err = repo.MergeableAmount(ctx, accountID, carded)
	if err != nil {
		t.Fatalf("MergeableAmount unique: %v", err)
	}
	if found {
		t.Errorf("unique item found = true, want false")
	}

	missing := domain.StashItem{NameID: 999, Amount: 1}
	_, found, err = repo.MergeableAmount(ctx, accountID, missing)
	if err != nil {
		t.Fatalf("MergeableAmount missing: %v", err)
	}
	if found {
		t.Errorf("missing item found = true, want false")
	}
}

func TestEscrowRepository_MoveToEscrow_RequiresLock(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewEscrowRepository(db, 0, 0)
	accountID := 9900040
	listingRef := int64(9900040001)
	cleanupAccounts(t, db, accountID)
	cleanupRefs(t, db, listingRef)

	setLock(t, db, accountID, false)
	stashID := insertStashRowForTest(t, db, accountID, 501, 10)

	ctx := context.Background()
	moves := []domain.EscrowMove{{StashItemID: stashID, Amount: 10}}

	err := repo.MoveToEscrow(ctx, accountID, listingRef, moves)
	if !errors.Is(err, domain.ErrStorageUnlocked) {
		t.Fatalf("MoveToEscrow err = %v, want ErrStorageUnlocked", err)
	}

	stashCount := countRows(t, db, "SELECT COUNT(*) FROM cp_storage WHERE account_id = ?", accountID)
	if stashCount != 1 {
		t.Errorf("stash count = %d, want 1 (untouched)", stashCount)
	}
}

func TestEscrowRepository_MoveToEscrow_FullAndPartial(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewEscrowRepository(db, 0, 0)
	accountID := 9900050
	fullRef := int64(9900050001)
	partialRef := int64(9900050002)
	boundRef := int64(9900050003)
	cleanupAccounts(t, db, accountID)
	cleanupRefs(t, db, fullRef, partialRef, boundRef)

	setLock(t, db, accountID, true)
	ctx := context.Background()

	nonStackable := insertStashRowForTest(t, db, accountID, 1201, 1, withUniqueID(700001))
	err := repo.MoveToEscrow(ctx, accountID, fullRef, []domain.EscrowMove{{StashItemID: nonStackable, Amount: 1}})
	if err != nil {
		t.Fatalf("MoveToEscrow full: %v", err)
	}

	stashCount := countRows(t, db, "SELECT COUNT(*) FROM cp_storage WHERE id = ?", nonStackable)
	if stashCount != 0 {
		t.Errorf("full move stash row count = %d, want 0", stashCount)
	}

	escrowAmount := -1
	if err := db.QueryRow("SELECT amount FROM cp_storage_escrow WHERE listing_ref = ?", fullRef).Scan(&escrowAmount); err != nil {
		t.Fatalf("scan full escrow amount: %v", err)
	}
	if escrowAmount != 1 {
		t.Errorf("full escrow amount = %d, want 1", escrowAmount)
	}

	stackable := insertStashRowForTest(t, db, accountID, 501, 10)
	err = repo.MoveToEscrow(ctx, accountID, partialRef, []domain.EscrowMove{{StashItemID: stackable, Amount: 4}})
	if err != nil {
		t.Fatalf("MoveToEscrow partial: %v", err)
	}

	remaining := -1
	if err := db.QueryRow("SELECT amount FROM cp_storage WHERE id = ?", stackable).Scan(&remaining); err != nil {
		t.Fatalf("scan partial stash amount: %v", err)
	}
	if remaining != 6 {
		t.Errorf("partial stash remaining = %d, want 6", remaining)
	}

	partialEscrow := -1
	if err := db.QueryRow("SELECT amount FROM cp_storage_escrow WHERE listing_ref = ?", partialRef).Scan(&partialEscrow); err != nil {
		t.Fatalf("scan partial escrow amount: %v", err)
	}
	if partialEscrow != 4 {
		t.Errorf("partial escrow amount = %d, want 4", partialEscrow)
	}

	boundItem := insertStashRowForTest(t, db, accountID, 502, 5, withBound(1))
	err = repo.MoveToEscrow(ctx, accountID, boundRef, []domain.EscrowMove{{StashItemID: boundItem, Amount: 5}})
	if !errors.Is(err, domain.ErrNotTradable) {
		t.Fatalf("MoveToEscrow bound err = %v, want ErrNotTradable", err)
	}

	boundStash := -1
	if err := db.QueryRow("SELECT amount FROM cp_storage WHERE id = ?", boundItem).Scan(&boundStash); err != nil {
		t.Fatalf("scan bound stash amount: %v", err)
	}
	if boundStash != 5 {
		t.Errorf("bound stash amount = %d, want 5 (unchanged)", boundStash)
	}

	boundEscrowCount := countRows(t, db, "SELECT COUNT(*) FROM cp_storage_escrow WHERE listing_ref = ?", boundRef)
	if boundEscrowCount != 0 {
		t.Errorf("bound escrow count = %d, want 0 (rolled back)", boundEscrowCount)
	}
}

func TestEscrowRepository_ReturnToStash(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewEscrowRepository(db, 0, 0)
	accountID := 9900060
	listingRef := int64(9900060001)
	cleanupAccounts(t, db, accountID)
	cleanupRefs(t, db, listingRef)

	setLock(t, db, accountID, true)
	ctx := context.Background()

	existing := insertStashRowForTest(t, db, accountID, 501, 4)
	moved := insertStashRowForTest(t, db, accountID, 501, 6)

	err := repo.MoveToEscrow(ctx, accountID, listingRef, []domain.EscrowMove{{StashItemID: moved, Amount: 6}})
	if err != nil {
		t.Fatalf("MoveToEscrow: %v", err)
	}

	if err := repo.ReturnToStash(ctx, listingRef); err != nil {
		t.Fatalf("ReturnToStash: %v", err)
	}

	mergedAmount := -1
	if err := db.QueryRow("SELECT amount FROM cp_storage WHERE id = ?", existing).Scan(&mergedAmount); err != nil {
		t.Fatalf("scan merged stash amount: %v", err)
	}
	if mergedAmount != 10 {
		t.Errorf("merged stash amount = %d, want 10", mergedAmount)
	}

	escrowCount := countRows(t, db, "SELECT COUNT(*) FROM cp_storage_escrow WHERE listing_ref = ?", listingRef)
	if escrowCount != 0 {
		t.Errorf("escrow count = %d, want 0", escrowCount)
	}

	if err := repo.ReturnToStash(ctx, listingRef); err != nil {
		t.Fatalf("ReturnToStash second call: %v", err)
	}
}

func TestEscrowRepository_Deliver_Idempotent(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewEscrowRepository(db, 0, 0)
	sellerID := 9900070
	recipientID := 9900071
	unlockedRecipient := 9900072
	listingRef := int64(9900070001)
	otherRef := int64(9900070002)
	legID := int64(9900070100)
	cleanupAccounts(t, db, sellerID, recipientID, unlockedRecipient)
	cleanupRefs(t, db, listingRef, otherRef)
	cleanupLegs(t, db, legID)

	setLock(t, db, sellerID, true)
	setLock(t, db, recipientID, true)
	setLock(t, db, unlockedRecipient, false)
	ctx := context.Background()

	moved := insertStashRowForTest(t, db, sellerID, 501, 8)
	if err := repo.MoveToEscrow(ctx, sellerID, listingRef, []domain.EscrowMove{{StashItemID: moved, Amount: 8}}); err != nil {
		t.Fatalf("MoveToEscrow: %v", err)
	}

	if err := repo.Deliver(ctx, listingRef, recipientID, legID); err != nil {
		t.Fatalf("Deliver: %v", err)
	}

	deliveredAmount := -1
	if err := db.QueryRow("SELECT amount FROM cp_storage WHERE account_id = ? AND nameid = ?", recipientID, 501).Scan(&deliveredAmount); err != nil {
		t.Fatalf("scan delivered amount: %v", err)
	}
	if deliveredAmount != 8 {
		t.Errorf("delivered amount = %d, want 8", deliveredAmount)
	}

	if err := repo.Deliver(ctx, listingRef, recipientID, legID); err != nil {
		t.Fatalf("Deliver second call: %v", err)
	}

	rowCount := countRows(t, db, "SELECT COUNT(*) FROM cp_storage WHERE account_id = ? AND nameid = ?", recipientID, 501)
	if rowCount != 1 {
		t.Errorf("recipient row count after second deliver = %d, want 1", rowCount)
	}

	afterAmount := -1
	if err := db.QueryRow("SELECT amount FROM cp_storage WHERE account_id = ? AND nameid = ?", recipientID, 501).Scan(&afterAmount); err != nil {
		t.Fatalf("scan amount after second deliver: %v", err)
	}
	if afterAmount != 8 {
		t.Errorf("recipient amount after second deliver = %d, want 8", afterAmount)
	}

	moved2 := insertStashRowForTest(t, db, sellerID, 502, 3)
	if err := repo.MoveToEscrow(ctx, sellerID, otherRef, []domain.EscrowMove{{StashItemID: moved2, Amount: 3}}); err != nil {
		t.Fatalf("MoveToEscrow other: %v", err)
	}

	err := repo.Deliver(ctx, otherRef, unlockedRecipient, 0)
	if !errors.Is(err, domain.ErrStorageUnlocked) {
		t.Errorf("Deliver to unlocked err = %v, want ErrStorageUnlocked", err)
	}
}

func TestEscrowRepository_Deliver_StorageFull(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewEscrowRepository(db, 1, 0)
	sellerID := 9900080
	recipientID := 9900081
	listingRef := int64(9900080001)
	cleanupAccounts(t, db, sellerID, recipientID)
	cleanupRefs(t, db, listingRef)

	setLock(t, db, sellerID, true)
	setLock(t, db, recipientID, true)
	ctx := context.Background()

	insertStashRowForTest(t, db, recipientID, 1201, 1, withUniqueID(800001))

	moved := insertStashRowForTest(t, db, sellerID, 1201, 1, withUniqueID(800002))
	if err := repo.MoveToEscrow(ctx, sellerID, listingRef, []domain.EscrowMove{{StashItemID: moved, Amount: 1}}); err != nil {
		t.Fatalf("MoveToEscrow: %v", err)
	}

	err := repo.Deliver(ctx, listingRef, recipientID, 0)
	if !errors.Is(err, domain.ErrStorageFull) {
		t.Fatalf("Deliver err = %v, want ErrStorageFull", err)
	}

	escrowCount := countRows(t, db, "SELECT COUNT(*) FROM cp_storage_escrow WHERE listing_ref = ?", listingRef)
	if escrowCount != 1 {
		t.Errorf("escrow count = %d, want 1 (rolled back)", escrowCount)
	}
}
