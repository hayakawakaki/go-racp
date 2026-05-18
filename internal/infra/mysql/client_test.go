package mysql

import (
	"os"
	"strings"
	"testing"

	"github.com/hayakawakaki/go-racp/server/config"
)

func TestConfigure_ForcesParseTime(t *testing.T) {
	t.Parallel()

	cfg, err := configure("user:pass@tcp(127.0.0.1:3306)/main")
	if err != nil {
		t.Fatalf("configure: %v", err)
	}
	if !cfg.ParseTime {
		t.Errorf("ParseTime = false, want true")
	}
}

func TestConfigure_PreservesOtherParams(t *testing.T) {
	t.Parallel()

	cfg, err := configure("user:pass@tcp(127.0.0.1:3306)/main?charset=utf8mb4&loc=Asia%2FTokyo")
	if err != nil {
		t.Fatalf("configure: %v", err)
	}
	if !cfg.ParseTime {
		t.Errorf("ParseTime = false, want true")
	}
	got := cfg.FormatDSN()
	for _, want := range []string{"charset=utf8mb4", "loc=Asia%2FTokyo"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in DSN: %q", want, got)
		}
	}
}

func TestConfigure_MalformedReturnsError(t *testing.T) {
	t.Parallel()

	_, err := configure("not-a-dsn")
	if err == nil {
		t.Fatal("expected error for malformed DSN, got nil")
	}
	if !strings.Contains(err.Error(), "ParseDSN") {
		t.Errorf("error missing ParseDSN prefix: %v", err)
	}
}

func TestConnect(t *testing.T) {
	env := envFromOS(t)

	main, logs := Connect(env)
	t.Cleanup(func() {
		_ = main.Close()
		_ = logs.Close()
	})

	if err := main.Ping(); err != nil {
		t.Fatalf("main db ping: %v", err)
	}
	if err := logs.Ping(); err != nil {
		t.Fatalf("log db ping: %v", err)
	}
}

func TestAttemptConnect_BadMainURL(t *testing.T) {
	env := envFromOS(t)
	env.DBMainURL = "user:pass@tcp(127.0.0.1:1)/nope?timeout=1s"

	if _, _, err := attemptConnect(env); err == nil {
		t.Fatal("expected error for unreachable main db, got nil")
	}
}

func TestAttemptConnect_BadLogURL(t *testing.T) {
	env := envFromOS(t)
	env.DBLogURL = "user:pass@tcp(127.0.0.1:1)/nope?timeout=1s"

	if _, _, err := attemptConnect(env); err == nil {
		t.Fatal("expected error for unreachable log db, got nil")
	}
}

func envFromOS(t *testing.T) *config.EnvConfig {
	t.Helper()

	main := os.Getenv("DB_MAIN_URL")
	logs := os.Getenv("DB_LOG_URL")
	if main == "" || logs == "" {
		t.Skip("DB_MAIN_URL and DB_LOG_URL must be set for integration tests")
	}

	return &config.EnvConfig{
		DBMainURL:     main,
		DBLogURL:      logs,
		DBMaxOpenConn: 4,
		DBMaxIdleConn: 4,
	}
}
