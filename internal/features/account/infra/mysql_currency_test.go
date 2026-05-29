//go:build integration

package infra

import (
	"context"
	"testing"

	"github.com/hayakawakaki/go-racp/internal/testutil"
)

func TestDepositQueue_BatchOrdersByIDAndLimits(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	testutil.TruncateMariaDB(t, db, "cp_deposit")
	queue := NewDepositQueue(db)
	ctx := context.Background()

	if _, err := db.ExecContext(ctx,
		"INSERT INTO cp_deposit (id, account_id, zeny, points) VALUES (3, 42, 30, 3), (1, 42, 10, 1), (2, 42, 20, 2)"); err != nil {
		t.Fatalf("seed: %v", err)
	}

	rows, err := queue.Batch(ctx, 2)
	if err != nil {
		t.Fatalf("Batch: %v", err)
	}
	if len(rows) != 2 || rows[0].ID != 1 || rows[1].ID != 2 {
		t.Errorf("Batch(2) = %+v, want ids [1 2] in order", rows)
	}

	all, err := queue.Batch(ctx, 10)
	if err != nil {
		t.Fatalf("Batch: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("Batch(10) returned %d rows, want 3", len(all))
	}
}

func TestDepositQueue_Delete(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	testutil.TruncateMariaDB(t, db, "cp_deposit")
	queue := NewDepositQueue(db)
	ctx := context.Background()

	if _, err := db.ExecContext(ctx,
		"INSERT INTO cp_deposit (id, account_id, zeny, points) VALUES (5, 42, 50, 5)"); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := queue.Delete(ctx, 5); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	rows, err := queue.Batch(ctx, 10)
	if err != nil {
		t.Fatalf("Batch: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("Batch after Delete = %+v, want none", rows)
	}
}

func TestRepository_EmailsByIDs(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewRepository(db)
	ctx := context.Background()

	suf := randomizeSuffix(t)
	first := createSeedUser(t, repo, "mail_a_"+suf, "mail_a_"+suf+"@example.invalid")
	second := createSeedUser(t, repo, "mail_b_"+suf, "mail_b_"+suf+"@example.invalid")

	emails, err := repo.EmailsByIDs(ctx, []int{first.ID, second.ID, -1})
	if err != nil {
		t.Fatalf("EmailsByIDs: %v", err)
	}
	if emails[first.ID] != first.Email {
		t.Errorf("email[%d] = %q, want %q", first.ID, emails[first.ID], first.Email)
	}
	if emails[second.ID] != second.Email {
		t.Errorf("email[%d] = %q, want %q", second.ID, emails[second.ID], second.Email)
	}
	if _, ok := emails[-1]; ok {
		t.Errorf("unknown id -1 must be absent from map")
	}
}

func TestRepository_EmailsByIDs_Empty(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewRepository(db)

	emails, err := repo.EmailsByIDs(context.Background(), nil)
	if err != nil {
		t.Fatalf("EmailsByIDs: %v", err)
	}
	if len(emails) != 0 {
		t.Errorf("emails = %+v, want empty for no ids", emails)
	}
}

func TestWithdrawQueue_InsertIsIdempotent(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	testutil.TruncateMariaDB(t, db, "cp_withdraw")
	queue := NewWithdrawQueue(db)
	ctx := context.Background()

	if err := queue.Insert(ctx, 7, 42, 1000, 50); err != nil {
		t.Fatalf("first Insert: %v", err)
	}
	if err := queue.Insert(ctx, 7, 42, 1000, 50); err != nil {
		t.Fatalf("second Insert: %v", err)
	}

	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM cp_withdraw WHERE id = 7").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("cp_withdraw rows for id 7 = %d, want 1 (duplicate insert must be a no-op)", count)
	}
}

func TestWithdrawQueue_DeliveredAndDelete(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	testutil.TruncateMariaDB(t, db, "cp_withdraw")
	queue := NewWithdrawQueue(db)
	ctx := context.Background()

	const epoch int64 = 1700000000
	if _, err := db.ExecContext(ctx,
		"INSERT INTO cp_withdraw (id, account_id, zeny, points, delivered_at) VALUES (7, 42, 0, 0, ?), (8, 42, 100, 0, 0)",
		epoch); err != nil {
		t.Fatalf("seed: %v", err)
	}

	rows, err := queue.Delivered(ctx, 10)
	if err != nil {
		t.Fatalf("Delivered: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("Delivered = %+v, want only the delivered_at > 0 row", rows)
	}
	if rows[0].ID != 7 || rows[0].DeliveredAt != epoch || rows[0].Zeny != 0 || rows[0].Points != 0 {
		t.Errorf("Delivered row = %+v, want {ID:7 DeliveredAt:%d Zeny:0 Points:0}", rows[0], epoch)
	}

	if err := queue.Delete(ctx, 7); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	rows, err = queue.Delivered(ctx, 10)
	if err != nil {
		t.Fatalf("Delivered after Delete: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("Delivered after Delete = %+v, want none", rows)
	}
}

func TestWithdrawQueue_ResetDelivered(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	testutil.TruncateMariaDB(t, db, "cp_withdraw")
	queue := NewWithdrawQueue(db)
	ctx := context.Background()

	const epoch int64 = 1700000000
	if _, err := db.ExecContext(ctx,
		"INSERT INTO cp_withdraw (id, account_id, zeny, points, delivered_at) VALUES (7, 42, 0, 0, ?)",
		epoch); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := queue.ResetDelivered(ctx, 7); err != nil {
		t.Fatalf("ResetDelivered: %v", err)
	}

	rows, err := queue.Delivered(ctx, 10)
	if err != nil {
		t.Fatalf("Delivered after ResetDelivered: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("Delivered after ResetDelivered = %+v, want none (delivered_at back to 0)", rows)
	}
}
