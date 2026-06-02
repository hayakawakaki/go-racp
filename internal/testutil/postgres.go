package testutil

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// OpenPostgres opens a pgx pool from the DSN in envVar, pings it, and registers cleanup. The test is skipped when the variable is empty.
func OpenPostgres(t *testing.T, envVar string) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv(envVar)
	if dsn == "" {
		t.Skipf("%s not set; skipping integration test", envVar)
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("pool.Ping: %v", err)
	}

	t.Cleanup(func() { pool.Close() })

	return pool
}

// TruncatePostgres truncates table with RESTART IDENTITY CASCADE after validating the identifier. The test fails on an invalid name or SQL error.
func TruncatePostgres(t *testing.T, pool *pgxpool.Pool, table string) {
	t.Helper()

	if !validIdentifier.MatchString(table) {
		t.Fatalf("invalid table identifier: %q", table)
	}

	if _, err := pool.Exec(context.Background(),
		fmt.Sprintf(`TRUNCATE TABLE %q RESTART IDENTITY CASCADE`, table)); err != nil {
		t.Fatalf("TRUNCATE %s: %v", table, err)
	}
}

const CurrencyLockKey int64 = 4242

func LockPostgres(t *testing.T, pool *pgxpool.Pool, key int64) {
	t.Helper()

	ctx := context.Background()

	connection, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("pool.Acquire: %v", err)
	}

	if _, err := connection.Exec(ctx, `SELECT pg_advisory_lock($1)`, key); err != nil {
		connection.Release()
		t.Fatalf("pg_advisory_lock: %v", err)
	}

	t.Cleanup(func() {
		_, _ = connection.Exec(ctx, `SELECT pg_advisory_unlock($1)`, key)
		connection.Release()
	})
}
