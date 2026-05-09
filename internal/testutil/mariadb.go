package testutil

import (
	"database/sql"
	"fmt"
	"os"
	"regexp"
	"testing"

	gomysql "github.com/go-sql-driver/mysql"
)

var validIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// OpenMariaDB obtains a MariaDB DSN from envVar, opens and verifies a *sql.DB for use in tests, and registers cleanup to close it.
// It skips the test if the environment variable is empty, enables time parsing on the connection, and fails the test on configuration or connectivity errors.
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

// TruncateMariaDB truncates the named table in the provided MariaDB database for use in tests.
// It validates the table identifier and fails the test if the name is invalid or if the TRUNCATE statement fails.
func TruncateMariaDB(t *testing.T, db *sql.DB, table string) {
	t.Helper()
	if !validIdentifier.MatchString(table) {
		t.Fatalf("invalid table identifier: %q", table)
	}
	if _, err := db.Exec(fmt.Sprintf("TRUNCATE TABLE `%s`", table)); err != nil {
		t.Fatalf("TRUNCATE %s: %v", table, err)
	}
}
