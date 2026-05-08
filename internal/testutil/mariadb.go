package testutil

import (
	"database/sql"
	"fmt"
	"os"
	"testing"

	gomysql "github.com/go-sql-driver/mysql"
)

func OpenMariaDB(t *testing.T, envVar string) *sql.DB {
	t.Helper()
	dsn := os.Getenv(envVar)
	if dsn == "" {
		t.Skipf("%s not set; skipping integration test", envVar)
	}
	cfg, err := gomysql.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("ParseDSN: %v", err)
	}
	cfg.ParseTime = true
	connector, err := gomysql.NewConnector(cfg)
	if err != nil {
		t.Fatalf("NewConnector: %v", err)
	}
	db := sql.OpenDB(connector)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		t.Fatalf("db.Ping: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TruncateMariaDB(t *testing.T, db *sql.DB, table string) {
	t.Helper()
	if _, err := db.Exec(fmt.Sprintf("TRUNCATE TABLE %s", table)); err != nil {
		t.Fatalf("TRUNCATE %s: %v", table, err)
	}
}
