package infra

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type LoginAttemptsRepository struct {
	Pool *pgxpool.Pool
}

func NewLoginAttemptsRepository(pool *pgxpool.Pool) *LoginAttemptsRepository {
	return &LoginAttemptsRepository{Pool: pool}
}

func (r *LoginAttemptsRepository) Record(ctx context.Context, username string, accountID sql.NullInt64, ip net.IP, success bool, userAgent string) error {
	addr := ip
	if addr == nil {
		addr = net.IPv4zero
	}

	_, err := r.Pool.Exec(ctx,
		`INSERT INTO cp_login_attempts (username, account_id, ip, success, user_agent)
		 VALUES ($1, $2, $3, $4, $5)`,
		username, accountID, addr.String(), success, userAgent,
	)
	if err != nil {
		return fmt.Errorf("infra.LoginAttemptsRepository.Record: %w", err)
	}

	return nil
}

func (r *LoginAttemptsRepository) ConsecutiveFailures(ctx context.Context, username string, window time.Duration) (int, time.Time, error) {
	key := strings.ToLower(username)
	cutoff := time.Now().Add(-window)

	var (
		count    int
		lastFail time.Time
	)
	err := r.Pool.QueryRow(ctx,
		`SELECT COUNT(*), COALESCE(MAX(attempted_at), 'epoch'::timestamptz)
		 FROM cp_login_attempts
		 WHERE LOWER(username) = $1
		   AND attempted_at > $2
		   AND success = FALSE`,
		key, cutoff,
	).Scan(&count, &lastFail)
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("infra.LoginAttemptsRepository.ConsecutiveFailures: %w", err)
	}

	return count, lastFail, nil
}
