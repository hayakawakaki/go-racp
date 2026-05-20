package infra

import (
	"context"
	"fmt"
	"time"

	"github.com/hayakawakaki/go-racp/internal/platform/metric/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PeakRepository struct {
	Pool *pgxpool.Pool
}

func NewPeakRepository(pool *pgxpool.Pool) *PeakRepository {
	return &PeakRepository{Pool: pool}
}

func (r *PeakRepository) UpsertIfGreater(
	ctx context.Context,
	metric domain.MetricName,
	window domain.Window,
	windowKey time.Time,
	value int,
) error {
	_, err := r.Pool.Exec(ctx,
		`INSERT INTO cp_metric_peak (metric, period, window_key, value, occurred_at)
		 VALUES ($1, $2, $3, $4, NOW())
		 ON CONFLICT (metric, period, window_key)
		 DO UPDATE SET value = EXCLUDED.value, occurred_at = NOW()
		 WHERE EXCLUDED.value > cp_metric_peak.value`,
		string(metric), string(window), windowKey, value,
	)
	if err != nil {
		return fmt.Errorf("infra.PeakRepository.UpsertIfGreater: %w", err)
	}
	return nil
}

func (r *PeakRepository) Current(
	ctx context.Context,
	keys map[domain.Window]time.Time,
) ([]domain.PeakRow, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	windows := make([]string, 0, len(keys))
	windowKeys := make([]time.Time, 0, len(keys))
	for w, k := range keys {
		windows = append(windows, string(w))
		windowKeys = append(windowKeys, k)
	}

	rows, err := r.Pool.Query(ctx,
		`SELECT metric, period, window_key, value, occurred_at
		 FROM cp_metric_peak
		 WHERE (period, window_key) IN (
		     SELECT UNNEST($1::text[]), UNNEST($2::date[])
		 )
		 ORDER BY metric, period`,
		windows, windowKeys,
	)
	if err != nil {
		return nil, fmt.Errorf("infra.PeakRepository.Current: %w", err)
	}
	defer rows.Close()

	out := make([]domain.PeakRow, 0)
	for rows.Next() {
		var pr domain.PeakRow
		var metric, window string
		if err := rows.Scan(&metric, &window, &pr.WindowKey, &pr.Value, &pr.OccurredAt); err != nil {
			return nil, fmt.Errorf("infra.PeakRepository.Current scan: %w", err)
		}
		pr.Metric = domain.MetricName(metric)
		pr.Window = domain.Window(window)
		out = append(out, pr)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("infra.PeakRepository.Current rows: %w", err)
	}
	return out, nil
}
