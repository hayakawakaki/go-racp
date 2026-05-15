package postgres

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hayakawakaki/go-racp/server/config"
)

const (
	connMaxLifetime     = 5 * time.Minute
	connMaxIdleTime     = 1 * time.Minute
	connRetryInterval   = 3 * time.Second
	connMaxRetryAttempt = 5
	pingTimeout         = 5 * time.Second
)

// Connect opens the control-panel Postgres pool, retrying on failure and terminating the process if all attempts are exhausted.
func Connect(env *config.EnvConfig) *pgxpool.Pool {
	fmt.Println("connecting to Postgres CP DB...")

	for i := 1; i <= connMaxRetryAttempt; i++ {
		pool, err := attemptConnect(env)
		if err == nil {
			fmt.Println("connected to Postgres successfully.")
			return pool
		}

		log.Printf("Postgres connection attempt %d/%d failed: %v", i, connMaxRetryAttempt, err)

		if i < connMaxRetryAttempt {
			time.Sleep(connRetryInterval)
		}
	}

	log.Fatalf("unable to connect to Postgres after %d attempts", connMaxRetryAttempt)

	return nil
}

func attemptConnect(env *config.EnvConfig) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(env.DBCPURL)
	if err != nil {
		return nil, fmt.Errorf("ParseConfig: %w", err)
	}

	cfg.MaxConns = int32(env.DBCPMaxOpenConn) //nolint:gosec // pool size is env-configured
	cfg.MinConns = int32(env.DBCPMaxIdleConn) //nolint:gosec // pool size is env-configured
	cfg.MaxConnLifetime = connMaxLifetime
	cfg.MaxConnIdleTime = connMaxIdleTime

	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("NewWithConfig: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pool.Ping: %w", err)
	}

	return pool, nil
}

type HealthPinger struct {
	pool *pgxpool.Pool
}

func NewHealthPinger(pool *pgxpool.Pool) HealthPinger {
	return HealthPinger{pool: pool}
}

func (p HealthPinger) PingContext(ctx context.Context) error {
	if err := p.pool.Ping(ctx); err != nil {
		return fmt.Errorf("pgxpool ping: %w", err)
	}
	return nil
}
