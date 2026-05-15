//go:build integration

package infra

import (
	"context"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/account/domain"
	"github.com/hayakawakaki/go-racp/internal/testutil"
)

var _ domain.ChangeLog = (*ChangeLogRepository)(nil)

func TestChangeLogRepository_RecordAndMostRecent(t *testing.T) {
	pool := testutil.OpenPostgres(t, "DB_CP_TEST_URL")
	testutil.TruncatePostgres(t, pool, "cp_account_record")
	repo := NewChangeLogRepository(pool)
	ctx := context.Background()

	at := time.Now().UTC().Truncate(time.Second)
	if err := repo.Record(ctx, 1, domain.ChangeTypePassword, at); err != nil {
		t.Fatalf("Record: %v", err)
	}

	got, err := repo.MostRecent(ctx, 1, domain.ChangeTypePassword)
	if err != nil {
		t.Fatalf("MostRecent: %v", err)
	}
	if !got.Equal(at) {
		t.Errorf("got %v, want %v", got, at)
	}
}

func TestChangeLogRepository_Record_UpsertsLatestTime(t *testing.T) {
	pool := testutil.OpenPostgres(t, "DB_CP_TEST_URL")
	testutil.TruncatePostgres(t, pool, "cp_account_record")
	repo := NewChangeLogRepository(pool)
	ctx := context.Background()

	first := time.Now().UTC().Truncate(time.Second).Add(-time.Hour)
	second := first.Add(time.Hour)
	if err := repo.Record(ctx, 1, domain.ChangeTypeEmail, first); err != nil {
		t.Fatalf("first Record: %v", err)
	}
	if err := repo.Record(ctx, 1, domain.ChangeTypeEmail, second); err != nil {
		t.Fatalf("second Record: %v", err)
	}

	got, err := repo.MostRecent(ctx, 1, domain.ChangeTypeEmail)
	if err != nil {
		t.Fatalf("MostRecent: %v", err)
	}
	if !got.Equal(second) {
		t.Errorf("got %v, want %v (upsert should keep latest)", got, second)
	}
}

func TestChangeLogRepository_MostRecent_NoRowsReturnsZeroTime(t *testing.T) {
	pool := testutil.OpenPostgres(t, "DB_CP_TEST_URL")
	testutil.TruncatePostgres(t, pool, "cp_account_record")
	repo := NewChangeLogRepository(pool)

	got, err := repo.MostRecent(context.Background(), 999, domain.ChangeTypePassword)
	if err != nil {
		t.Fatalf("MostRecent: %v", err)
	}
	if !got.IsZero() {
		t.Errorf("got %v, want zero time", got)
	}
}

func TestChangeLogRepository_MostRecent_FiltersByChangeType(t *testing.T) {
	pool := testutil.OpenPostgres(t, "DB_CP_TEST_URL")
	testutil.TruncatePostgres(t, pool, "cp_account_record")
	repo := NewChangeLogRepository(pool)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	if err := repo.Record(ctx, 1, domain.ChangeTypePassword, now); err != nil {
		t.Fatalf("Record password: %v", err)
	}
	if err := repo.Record(ctx, 1, domain.ChangeTypeEmail, now.Add(time.Hour)); err != nil {
		t.Fatalf("Record email: %v", err)
	}

	got, err := repo.MostRecent(ctx, 1, domain.ChangeTypePassword)
	if err != nil {
		t.Fatalf("MostRecent: %v", err)
	}
	if !got.Equal(now) {
		t.Errorf("got %v, want %v (email change should not affect password lookup)", got, now)
	}
}
