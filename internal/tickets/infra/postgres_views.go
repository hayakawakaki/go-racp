package infra

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ViewRepository struct {
	Pool *pgxpool.Pool
}

func NewViewRepository(pool *pgxpool.Pool) *ViewRepository {
	return &ViewRepository{Pool: pool}
}

func (r *ViewRepository) Get(ctx context.Context, accountID int, ticketID int64) (time.Time, error) {
	var at time.Time
	err := r.Pool.QueryRow(ctx,
		`SELECT last_viewed FROM cp_ticket_views WHERE account_id = $1 AND ticket_id = $2`,
		accountID, ticketID,
	).Scan(&at)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("infra.ViewRepository.Get: %w", err)
	}

	return at, nil
}

func (r *ViewRepository) Upsert(ctx context.Context, accountID int, ticketID int64, at time.Time) error {
	_, err := r.Pool.Exec(ctx,
		`INSERT INTO cp_ticket_views (account_id, ticket_id, last_viewed)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (account_id, ticket_id) DO UPDATE
		 SET last_viewed = GREATEST(cp_ticket_views.last_viewed, EXCLUDED.last_viewed)`,
		accountID, ticketID, at,
	)
	if err != nil {
		return fmt.Errorf("infra.ViewRepository.Upsert: %w", err)
	}

	return nil
}

func (r *ViewRepository) UnreadCountForPlayer(ctx context.Context, accountID int) (int, error) {
	var count int
	err := r.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM cp_tickets t
		 LEFT JOIN cp_ticket_views v ON v.ticket_id = t.id AND v.account_id = $1
		 WHERE t.account_id = $1 AND (v.last_viewed IS NULL OR t.last_activity > v.last_viewed)`,
		accountID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("infra.ViewRepository.UnreadCountForPlayer: %w", err)
	}

	return count, nil
}

func (r *ViewRepository) UnreadCountForStaff(ctx context.Context, accountID int, categoryKeys []string) (int, error) {
	if len(categoryKeys) == 0 {
		return 0, nil
	}
	args := []any{accountID}
	placeholders := make([]string, 0, len(categoryKeys))
	for i, key := range categoryKeys {
		placeholders = append(placeholders, "$"+strconv.Itoa(i+2))
		args = append(args, key)
	}
	query := `SELECT COUNT(*) FROM cp_tickets t
	          LEFT JOIN cp_ticket_views v ON v.ticket_id = t.id AND v.account_id = $1
	          WHERE t.category IN (` + strings.Join(placeholders, ",") + `)
	          AND (v.last_viewed IS NULL OR t.last_activity > v.last_viewed)`

	var count int
	if err := r.Pool.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("infra.ViewRepository.UnreadCountForStaff: %w", err)
	}

	return count, nil
}
