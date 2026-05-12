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

func TestMySQLRepository_RecordAndMostRecent(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	testutil.TruncateMariaDB(t, db, "cp_account_record")
	repo := NewChangeLogRepository(db)
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

func TestMySQLRepository_Record_UpsertsLatestTime(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	testutil.TruncateMariaDB(t, db, "cp_account_record")
	repo := NewChangeLogRepository(db)
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

func TestMySQLRepository_MostRecent_NoRowsReturnsZeroTime(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	testutil.TruncateMariaDB(t, db, "cp_account_record")
	repo := NewChangeLogRepository(db)

	got, err := repo.MostRecent(context.Background(), 999, domain.ChangeTypePassword)
	if err != nil {
		t.Fatalf("MostRecent: %v", err)
	}
	if !got.IsZero() {
		t.Errorf("got %v, want zero time", got)
	}
}

func TestMySQLRepository_MostRecent_FiltersByChangeType(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	testutil.TruncateMariaDB(t, db, "cp_account_record")
	repo := NewChangeLogRepository(db)
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
