package postgres

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/hayakawakaki/go-racp/server/config"
)

func TestAttemptConnect_MalformedDSNReturnsError(t *testing.T) {
	t.Parallel()

	env := &config.EnvConfig{
		DBCPURL:         "not-a-dsn",
		DBCPMaxOpenConn: 4,
		DBCPMaxIdleConn: 2,
	}

	pool, err := attemptConnect(env)
	if err == nil {
		if pool != nil {
			pool.Close()
		}
		t.Fatal("expected error for malformed DSN, got nil")
	}
	if !strings.Contains(err.Error(), "ParseConfig") {
		t.Errorf("error missing ParseConfig prefix: %v", err)
	}
}

func TestAttemptConnect_UnreachableReturnsError(t *testing.T) {
	t.Parallel()

	env := &config.EnvConfig{
		DBCPURL:         "postgres://nobody:nobody@127.0.0.1:1/none?connect_timeout=1",
		DBCPMaxOpenConn: 4,
		DBCPMaxIdleConn: 2,
	}

	pool, err := attemptConnect(env)
	if err == nil {
		if pool != nil {
			pool.Close()
		}
		t.Fatal("expected error for unreachable postgres, got nil")
	}
}

func TestConnect_Integration(t *testing.T) {
	dsn := os.Getenv("DB_CP_URL")
	if dsn == "" {
		t.Skip("DB_CP_URL must be set for integration tests")
	}

	env := &config.EnvConfig{
		DBCPURL:         dsn,
		DBCPMaxOpenConn: 4,
		DBCPMaxIdleConn: 2,
	}

	pool := Connect(env)
	t.Cleanup(func() { pool.Close() })

	if err := pool.Ping(context.Background()); err != nil {
		t.Fatalf("pool ping: %v", err)
	}
}

func TestHealthPinger_DelegatesPing(t *testing.T) {
	dsn := os.Getenv("DB_CP_URL")
	if dsn == "" {
		t.Skip("DB_CP_URL must be set for integration tests")
	}

	env := &config.EnvConfig{
		DBCPURL:         dsn,
		DBCPMaxOpenConn: 4,
		DBCPMaxIdleConn: 2,
	}

	pool := Connect(env)
	t.Cleanup(func() { pool.Close() })

	pinger := NewHealthPinger(pool)
	if err := pinger.PingContext(context.Background()); err != nil {
		t.Fatalf("pinger.PingContext: %v", err)
	}
}
