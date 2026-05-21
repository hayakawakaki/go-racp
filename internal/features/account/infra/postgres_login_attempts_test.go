//go:build integration

package infra

import (
	"context"
	"database/sql"
	"net"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/testutil"
)

func TestLoginAttemptsRepository_Record_InsertsRow(t *testing.T) {
	pool := testutil.OpenPostgres(t, "DB_CP_TEST_URL")
	testutil.TruncatePostgres(t, pool, "cp_login_attempts")
	repo := NewLoginAttemptsRepository(pool)
	ctx := context.Background()

	if err := repo.Record(ctx, "testuser", sql.NullInt64{Int64: 1, Valid: true}, net.IPv4(192, 0, 2, 1), false, "ua/1.0"); err != nil {
		t.Fatalf("Record: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM cp_login_attempts WHERE username = $1", "testuser").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
}

func TestLoginAttemptsRepository_Record_NilIPStoredAsZeroAddress(t *testing.T) {
	pool := testutil.OpenPostgres(t, "DB_CP_TEST_URL")
	testutil.TruncatePostgres(t, pool, "cp_login_attempts")
	repo := NewLoginAttemptsRepository(pool)
	ctx := context.Background()

	if err := repo.Record(ctx, "testuser", sql.NullInt64{}, nil, false, ""); err != nil {
		t.Fatalf("Record: %v", err)
	}

	var ip string
	if err := pool.QueryRow(ctx, "SELECT host(ip) FROM cp_login_attempts WHERE username = $1", "testuser").Scan(&ip); err != nil {
		t.Fatalf("select: %v", err)
	}
	if ip != "0.0.0.0" {
		t.Errorf("stored ip = %q, want 0.0.0.0", ip)
	}
}

func TestLoginAttemptsRepository_ConsecutiveFailures_EmptyTable(t *testing.T) {
	pool := testutil.OpenPostgres(t, "DB_CP_TEST_URL")
	testutil.TruncatePostgres(t, pool, "cp_login_attempts")
	repo := NewLoginAttemptsRepository(pool)

	count, lastFail, err := repo.ConsecutiveFailures(context.Background(), "missing", 15*time.Minute)
	if err != nil {
		t.Fatalf("ConsecutiveFailures: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
	if !lastFail.IsZero() && !lastFail.Equal(time.Unix(0, 0).UTC()) {
		t.Errorf("lastFail = %v, want zero or epoch", lastFail)
	}
}

func TestLoginAttemptsRepository_ConsecutiveFailures_CountsWithinWindow(t *testing.T) {
	pool := testutil.OpenPostgres(t, "DB_CP_TEST_URL")
	testutil.TruncatePostgres(t, pool, "cp_login_attempts")
	repo := NewLoginAttemptsRepository(pool)
	ctx := context.Background()

	for range 3 {
		if err := repo.Record(ctx, "testuser", sql.NullInt64{}, net.IPv4(192, 0, 2, 1), false, ""); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}

	count, _, err := repo.ConsecutiveFailures(ctx, "testuser", time.Minute)
	if err != nil {
		t.Fatalf("ConsecutiveFailures: %v", err)
	}
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}

func TestLoginAttemptsRepository_ConsecutiveFailures_ExcludesOutsideWindow(t *testing.T) {
	pool := testutil.OpenPostgres(t, "DB_CP_TEST_URL")
	testutil.TruncatePostgres(t, pool, "cp_login_attempts")
	ctx := context.Background()

	if _, err := pool.Exec(ctx,
		`INSERT INTO cp_login_attempts (username, ip, success, attempted_at) VALUES
		 ($1, '0.0.0.0'::inet, FALSE, NOW() - INTERVAL '2 hours'),
		 ($1, '0.0.0.0'::inet, FALSE, NOW())`,
		"testuser",
	); err != nil {
		t.Fatalf("seed: %v", err)
	}

	repo := NewLoginAttemptsRepository(pool)
	count, _, err := repo.ConsecutiveFailures(ctx, "testuser", time.Minute)
	if err != nil {
		t.Fatalf("ConsecutiveFailures: %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1 (only the recent row, the 2h-old one is outside window)", count)
	}
}

func TestLoginAttemptsRepository_ConsecutiveFailures_FiltersOutSuccesses(t *testing.T) {
	pool := testutil.OpenPostgres(t, "DB_CP_TEST_URL")
	testutil.TruncatePostgres(t, pool, "cp_login_attempts")
	repo := NewLoginAttemptsRepository(pool)
	ctx := context.Background()

	if err := repo.Record(ctx, "testuser", sql.NullInt64{}, net.IPv4(192, 0, 2, 1), false, ""); err != nil {
		t.Fatalf("Record failure: %v", err)
	}

	count, _, err := repo.ConsecutiveFailures(ctx, "testuser", time.Minute)
	if err != nil {
		t.Fatalf("ConsecutiveFailures: %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1 after the failure row", count)
	}
}

func TestLoginAttemptsRepository_ConsecutiveFailures_CaseInsensitiveUsername(t *testing.T) {
	pool := testutil.OpenPostgres(t, "DB_CP_TEST_URL")
	testutil.TruncatePostgres(t, pool, "cp_login_attempts")
	repo := NewLoginAttemptsRepository(pool)
	ctx := context.Background()

	if err := repo.Record(ctx, "TestUser", sql.NullInt64{}, net.IPv4(192, 0, 2, 1), false, ""); err != nil {
		t.Fatalf("Record: %v", err)
	}

	count, _, err := repo.ConsecutiveFailures(ctx, "TESTUSER", time.Minute)
	if err != nil {
		t.Fatalf("ConsecutiveFailures: %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1 (case-insensitive match)", count)
	}
}

func TestLoginAttemptsRepository_Record_SuccessClearsPriorFailures(t *testing.T) {
	pool := testutil.OpenPostgres(t, "DB_CP_TEST_URL")
	testutil.TruncatePostgres(t, pool, "cp_login_attempts")
	repo := NewLoginAttemptsRepository(pool)
	ctx := context.Background()

	for range 4 {
		if err := repo.Record(ctx, "testuser", sql.NullInt64{}, net.IPv4(192, 0, 2, 1), false, ""); err != nil {
			t.Fatalf("Record failure: %v", err)
		}
	}

	if err := repo.Record(ctx, "testuser", sql.NullInt64{Int64: 1, Valid: true}, net.IPv4(192, 0, 2, 1), true, ""); err != nil {
		t.Fatalf("Record success: %v", err)
	}

	count, _, err := repo.ConsecutiveFailures(ctx, "testuser", time.Minute)
	if err != nil {
		t.Fatalf("ConsecutiveFailures: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0 (success row should clear prior failures)", count)
	}

	var successCount int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM cp_login_attempts WHERE username = $1 AND success = TRUE", "testuser").Scan(&successCount); err != nil {
		t.Fatalf("count successes: %v", err)
	}
	if successCount != 1 {
		t.Errorf("success row count = %d, want 1 (success row must be preserved)", successCount)
	}
}

func TestLoginAttemptsRepository_DeleteOlderThan_RemovesOnlyOldRows(t *testing.T) {
	pool := testutil.OpenPostgres(t, "DB_CP_TEST_URL")
	testutil.TruncatePostgres(t, pool, "cp_login_attempts")
	ctx := context.Background()

	if _, err := pool.Exec(ctx,
		`INSERT INTO cp_login_attempts (username, ip, success, attempted_at) VALUES
		 ('old1',   '0.0.0.0'::inet, FALSE, NOW() - INTERVAL '40 days'),
		 ('old2',   '0.0.0.0'::inet, TRUE,  NOW() - INTERVAL '35 days'),
		 ('recent', '0.0.0.0'::inet, FALSE, NOW() - INTERVAL '1 day'),
		 ('now',    '0.0.0.0'::inet, FALSE, NOW())`,
	); err != nil {
		t.Fatalf("seed: %v", err)
	}

	repo := NewLoginAttemptsRepository(pool)
	cutoff := time.Now().Add(-30 * 24 * time.Hour)
	deleted, err := repo.DeleteOlderThan(ctx, cutoff)
	if err != nil {
		t.Fatalf("DeleteOlderThan: %v", err)
	}
	if deleted != 2 {
		t.Errorf("deleted = %d, want 2 (old1, old2)", deleted)
	}

	var remaining int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM cp_login_attempts").Scan(&remaining); err != nil {
		t.Fatalf("count: %v", err)
	}
	if remaining != 2 {
		t.Errorf("remaining = %d, want 2 (recent, now)", remaining)
	}
}

func TestLoginAttemptsRepository_DeleteOlderThan_EmptyTable(t *testing.T) {
	pool := testutil.OpenPostgres(t, "DB_CP_TEST_URL")
	testutil.TruncatePostgres(t, pool, "cp_login_attempts")
	repo := NewLoginAttemptsRepository(pool)

	deleted, err := repo.DeleteOlderThan(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("DeleteOlderThan: %v", err)
	}
	if deleted != 0 {
		t.Errorf("deleted = %d, want 0", deleted)
	}
}

func TestLoginAttemptsRepository_DeleteOlderThan_PreservesSuccessRows(t *testing.T) {
	pool := testutil.OpenPostgres(t, "DB_CP_TEST_URL")
	testutil.TruncatePostgres(t, pool, "cp_login_attempts")
	ctx := context.Background()

	if _, err := pool.Exec(ctx,
		`INSERT INTO cp_login_attempts (username, ip, success, attempted_at) VALUES
		 ('testuser', '0.0.0.0'::inet, TRUE,  NOW() - INTERVAL '1 hour'),
		 ('testuser', '0.0.0.0'::inet, FALSE, NOW() - INTERVAL '1 hour')`,
	); err != nil {
		t.Fatalf("seed: %v", err)
	}

	repo := NewLoginAttemptsRepository(pool)
	deleted, err := repo.DeleteOlderThan(ctx, time.Now())
	if err != nil {
		t.Fatalf("DeleteOlderThan: %v", err)
	}
	if deleted != 2 {
		t.Errorf("deleted = %d, want 2 (cutoff in the future deletes both)", deleted)
	}
}

func TestLoginAttemptsRepository_ConsecutiveFailures_ReturnsLastFailTimestamp(t *testing.T) {
	pool := testutil.OpenPostgres(t, "DB_CP_TEST_URL")
	testutil.TruncatePostgres(t, pool, "cp_login_attempts")
	repo := NewLoginAttemptsRepository(pool)
	ctx := context.Background()

	before := time.Now().UTC()
	if err := repo.Record(ctx, "testuser", sql.NullInt64{}, net.IPv4(192, 0, 2, 1), false, ""); err != nil {
		t.Fatalf("Record: %v", err)
	}
	after := time.Now().UTC()

	_, lastFail, err := repo.ConsecutiveFailures(ctx, "testuser", time.Minute)
	if err != nil {
		t.Fatalf("ConsecutiveFailures: %v", err)
	}
	if lastFail.Before(before.Add(-time.Second)) || lastFail.After(after.Add(time.Second)) {
		t.Errorf("lastFail = %v, want between %v and %v", lastFail, before, after)
	}
}
