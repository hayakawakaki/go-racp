package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RecordStore[K ~uint8] struct {
	pool      *pgxpool.Pool
	table     string
	insertSQL string
	selectSQL string
}

func NewRecordStore[K ~uint8](pool *pgxpool.Pool, table, idColumn string) *RecordStore[K] {
	return &RecordStore[K]{
		pool:  pool,
		table: table,
		insertSQL: fmt.Sprintf( //nolint:gosec // table and idColumn are caller-provided constants
			`INSERT INTO %s (%s, change_type, changed_at) VALUES ($1, $2, $3) `+
				`ON CONFLICT (%s, change_type) DO UPDATE SET changed_at = EXCLUDED.changed_at`,
			table, idColumn, idColumn,
		),
		selectSQL: fmt.Sprintf( //nolint:gosec // table and idColumn are caller-provided constants
			`SELECT changed_at FROM %s WHERE %s = $1 AND change_type = $2`,
			table, idColumn,
		),
	}
}

func (s *RecordStore[K]) Record(ctx context.Context, id int, kind K, at time.Time) error {
	if _, err := s.pool.Exec(ctx, s.insertSQL, id, kind, at); err != nil {
		return fmt.Errorf("postgres.RecordStore.Record(%s): %w", s.table, err)
	}

	return nil
}

func (s *RecordStore[K]) MostRecent(ctx context.Context, id int, kind K) (time.Time, error) {
	var changedAt time.Time
	err := s.pool.QueryRow(ctx, s.selectSQL, id, kind).Scan(&changedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("postgres.RecordStore.MostRecent(%s): %w", s.table, err)
	}

	return changedAt, nil
}
