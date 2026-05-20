//go:build integration

package infra

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"testing"

	"github.com/hayakawakaki/go-racp/internal/features/stall/domain"
	"github.com/hayakawakaki/go-racp/internal/testutil"
)

func randomSuffix(t *testing.T) string {
	t.Helper()
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatalf("rand: %v", err)
	}

	return hex.EncodeToString(b[:])
}

func insertChar(t *testing.T, db *sql.DB, name string) int {
	t.Helper()
	res, err := db.Exec("INSERT INTO `char` (account_id, name, sex) VALUES (?, ?, 'M')", 1, name)
	if err != nil {
		t.Fatalf("insert char %q: %v", name, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("char LastInsertId: %v", err)
	}
	charID := int(id)
	t.Cleanup(func() { _, _ = db.Exec("DELETE FROM `char` WHERE char_id = ?", charID) })

	return charID
}

func insertVending(t *testing.T, db *sql.DB, id, charID int, title, vmap string, x, y, autotrade int) {
	t.Helper()
	_, err := db.Exec(
		"INSERT INTO vendings (id, account_id, char_id, sex, map, x, y, title, autotrade) VALUES (?, 1, ?, 'M', ?, ?, ?, ?, ?)",
		id, charID, vmap, x, y, title, autotrade,
	)
	if err != nil {
		t.Fatalf("insert vendings id=%d: %v", id, err)
	}
	t.Cleanup(func() { _, _ = db.Exec("DELETE FROM vendings WHERE id = ?", id) })
}

func insertCartInventory(t *testing.T, db *sql.DB, charID, nameid, amount int) int {
	t.Helper()
	res, err := db.Exec(
		"INSERT INTO cart_inventory (char_id, nameid, amount) VALUES (?, ?, ?)",
		charID, nameid, amount,
	)
	if err != nil {
		t.Fatalf("insert cart_inventory: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("cart_inventory LastInsertId: %v", err)
	}
	ciID := int(id)
	t.Cleanup(func() { _, _ = db.Exec("DELETE FROM cart_inventory WHERE id = ?", ciID) })

	return ciID
}

func insertVendingItem(t *testing.T, db *sql.DB, vendingID, index, cartinventoryID, amount, price int) {
	t.Helper()
	_, err := db.Exec(
		"INSERT INTO vending_items (vending_id, `index`, cartinventory_id, amount, price) VALUES (?, ?, ?, ?, ?)",
		vendingID, index, cartinventoryID, amount, price,
	)
	if err != nil {
		t.Fatalf("insert vending_items: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM vending_items WHERE vending_id = ? AND `index` = ?", vendingID, index)
	})
}

func insertBuyingstore(t *testing.T, db *sql.DB, id, charID int, title, bmap string, x, y, autotrade, limit int) {
	t.Helper()
	_, err := db.Exec(
		"INSERT INTO buyingstores (id, account_id, char_id, sex, map, x, y, title, `limit`, autotrade) VALUES (?, 1, ?, 'M', ?, ?, ?, ?, ?, ?)",
		id, charID, bmap, x, y, title, limit, autotrade,
	)
	if err != nil {
		t.Fatalf("insert buyingstores id=%d: %v", id, err)
	}
	t.Cleanup(func() { _, _ = db.Exec("DELETE FROM buyingstores WHERE id = ?", id) })
}

func insertBuyingstoreItem(t *testing.T, db *sql.DB, buyingstoreID, index, itemID, amount, price int) {
	t.Helper()
	_, err := db.Exec(
		"INSERT INTO buyingstore_items (buyingstore_id, `index`, item_id, amount, price) VALUES (?, ?, ?, ?, ?)",
		buyingstoreID, index, itemID, amount, price,
	)
	if err != nil {
		t.Fatalf("insert buyingstore_items: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM buyingstore_items WHERE buyingstore_id = ? AND `index` = ?", buyingstoreID, index)
	})
}

func TestRepository_LoadAll_BothTypesAndSellerNames(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewRepository(db)
	suf := randomSuffix(t)

	sellerID := insertChar(t, db, "testuser_"+suf)
	buyerID := insertChar(t, db, "kaki_"+suf)
	vendingID := 1000000 + sellerID
	buyingID := 2000000 + buyerID

	insertVending(t, db, vendingID, sellerID, "selling-"+suf, "prontera", 100, 200, 1)
	ci := insertCartInventory(t, db, sellerID, 501, 10)
	insertVendingItem(t, db, vendingID, 0, ci, 5, 1500)

	insertBuyingstore(t, db, buyingID, buyerID, "buying-"+suf, "geffen", 50, 60, 0, 999999)
	insertBuyingstoreItem(t, db, buyingID, 0, 502, 7, 200)

	vendors, err := repo.LoadAll(context.Background())
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	var selling, buying *domain.Vendor
	for index := range vendors {
		switch vendors[index].Key() {
		case domain.VendorKey{Type: domain.VendorTypeSelling, ID: vendingID}:
			selling = &vendors[index]
		case domain.VendorKey{Type: domain.VendorTypeBuying, ID: buyingID}:
			buying = &vendors[index]
		}
	}

	if selling == nil {
		t.Fatalf("selling vendor %d missing from LoadAll", vendingID)
	}
	if selling.SellerName != "testuser_"+suf {
		t.Errorf("selling SellerName = %q, want testuser_%s", selling.SellerName, suf)
	}
	if selling.StallName != "selling-"+suf || selling.VendorMap != "prontera" {
		t.Errorf("selling stall = %+v", selling)
	}
	if len(selling.Items) != 1 || selling.Items[0].ItemID != 501 || selling.Items[0].Amount != 5 || selling.Items[0].Price != 1500 {
		t.Errorf("selling items = %+v", selling.Items)
	}

	if buying == nil {
		t.Fatalf("buying vendor %d missing from LoadAll", buyingID)
	}
	if buying.SellerName != "kaki_"+suf {
		t.Errorf("buying SellerName = %q", buying.SellerName)
	}
	if buying.BudgetLimit != 999999 {
		t.Errorf("buying BudgetLimit = %d, want 999999", buying.BudgetLimit)
	}
	if len(buying.Items) != 1 || buying.Items[0].ItemID != 502 || buying.Items[0].Amount != 7 || buying.Items[0].Price != 200 {
		t.Errorf("buying items = %+v", buying.Items)
	}
}

func TestRepository_LoadAll_OrphanCartInventoryDrops(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewRepository(db)
	suf := randomSuffix(t)

	sellerID := insertChar(t, db, "testuser_"+suf)
	vendingID := 3000000 + sellerID

	insertVending(t, db, vendingID, sellerID, "orphan-"+suf, "alberta", 1, 1, 1)
	_, err := db.Exec(
		"INSERT INTO vending_items (vending_id, `index`, cartinventory_id, amount, price) VALUES (?, 0, 999999999, 1, 1)",
		vendingID,
	)
	if err != nil {
		t.Fatalf("insert orphan vending_items: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM vending_items WHERE vending_id = ?", vendingID)
	})

	vendors, err := repo.LoadAll(context.Background())
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	for index := range vendors {
		if vendors[index].Key() == (domain.VendorKey{Type: domain.VendorTypeSelling, ID: vendingID}) {
			if len(vendors[index].Items) != 0 {
				t.Errorf("orphan vending should have 0 items, got %d", len(vendors[index].Items))
			}

			return
		}
	}
	t.Fatalf("vending %d missing from LoadAll", vendingID)
}
