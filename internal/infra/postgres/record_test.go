//go:build integration

package postgres_test

import (
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/infra/postgres"
	"github.com/hayakawakaki/go-racp/internal/testutil"
)

type testChangeType uint8

const (
	testChangeAlpha testChangeType = 1
	testChangeBeta  testChangeType = 2
)

func TestRecordStore(t *testing.T) {
	pool := testutil.OpenPostgres(t, "DB_CP_TEST_URL")
	testutil.TruncatePostgres(t, pool, "cp_account_record")
	store := postgres.NewRecordStore[testChangeType](pool, "cp_account_record", "account_id")
	ctx := t.Context()
	baseTime := time.Now().UTC().Truncate(time.Second)

	t.Run("record then most recent returns stored timestamp", func(t *testing.T) {
		t.Parallel()
		const id = 1001
		if err := store.Record(ctx, id, testChangeAlpha, baseTime); err != nil {
			t.Fatalf("Record: %v", err)
		}
		got, err := store.MostRecent(ctx, id, testChangeAlpha)
		if err != nil {
			t.Fatalf("MostRecent: %v", err)
		}
		if !got.Equal(baseTime) {
			t.Errorf("got %v, want %v", got, baseTime)
		}
	})

	t.Run("record upserts latest timestamp", func(t *testing.T) {
		t.Parallel()
		const id = 1002
		first := baseTime.Add(-time.Hour)
		second := baseTime
		if err := store.Record(ctx, id, testChangeAlpha, first); err != nil {
			t.Fatalf("Record first: %v", err)
		}
		if err := store.Record(ctx, id, testChangeAlpha, second); err != nil {
			t.Fatalf("Record second: %v", err)
		}
		got, err := store.MostRecent(ctx, id, testChangeAlpha)
		if err != nil {
			t.Fatalf("MostRecent: %v", err)
		}
		if !got.Equal(second) {
			t.Errorf("got %v, want %v (upsert should keep latest)", got, second)
		}
	})

	t.Run("most recent returns zero time when no row exists", func(t *testing.T) {
		t.Parallel()
		got, err := store.MostRecent(ctx, 9999, testChangeAlpha)
		if err != nil {
			t.Fatalf("MostRecent: %v", err)
		}
		if !got.IsZero() {
			t.Errorf("got %v, want zero time", got)
		}
	})

	t.Run("most recent filters by change kind", func(t *testing.T) {
		t.Parallel()
		const id = 1003
		alphaAt := baseTime
		betaAt := baseTime.Add(time.Hour)
		if err := store.Record(ctx, id, testChangeAlpha, alphaAt); err != nil {
			t.Fatalf("Record alpha: %v", err)
		}
		if err := store.Record(ctx, id, testChangeBeta, betaAt); err != nil {
			t.Fatalf("Record beta: %v", err)
		}
		got, err := store.MostRecent(ctx, id, testChangeAlpha)
		if err != nil {
			t.Fatalf("MostRecent: %v", err)
		}
		if !got.Equal(alphaAt) {
			t.Errorf("got %v, want %v (beta record should not affect alpha lookup)", got, alphaAt)
		}
	})

	t.Run("most recent isolates by id", func(t *testing.T) {
		t.Parallel()
		const recordedID = 1004
		const lookupID = 1005
		if err := store.Record(ctx, recordedID, testChangeAlpha, baseTime); err != nil {
			t.Fatalf("Record: %v", err)
		}
		got, err := store.MostRecent(ctx, lookupID, testChangeAlpha)
		if err != nil {
			t.Fatalf("MostRecent: %v", err)
		}
		if !got.IsZero() {
			t.Errorf("got %v, want zero time (lookupID has no row)", got)
		}
	})
}
