//go:build integration

package infra

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/go-sql-driver/mysql"
)

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DB_MAIN_URL")
	if dsn == "" {
		t.Skip("DB_MAIN_URL not set; skipping integration test")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		t.Fatalf("db.Ping: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// truncateSessions wipes the cp_sessions table. Safe because the table is
// owned by go-racp; rAthena does not touch it.
func truncateSessions(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec("TRUNCATE TABLE cp_sessions"); err != nil {
		t.Fatalf("TRUNCATE cp_sessions: %v", err)
	}
}
