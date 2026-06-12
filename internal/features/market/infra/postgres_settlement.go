package infra

import (
	"context"
	"fmt"

	"github.com/hayakawakaki/go-racp/internal/features/market/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SettlementRepository struct {
	Pool *pgxpool.Pool
}

func NewSettlementRepository(pool *pgxpool.Pool) *SettlementRepository {
	return &SettlementRepository{Pool: pool}
}

func (r *SettlementRepository) Enqueue(ctx context.Context, leg domain.SettlementLeg) error {
	_, err := r.Pool.Exec(ctx,
		`INSERT INTO cp_settlement (listing_id, escrow_ref, recipient_account_id, deliver_amount, whole, status)
		 VALUES ($1, $2, $3, $4, $5, 1)`,
		leg.ListingID, leg.EscrowRef, leg.RecipientAccountID, leg.DeliverAmount, leg.Whole,
	)
	if err != nil {
		return fmt.Errorf("infra.SettlementRepository.Enqueue: %w", err)
	}

	return nil
}

func (r *SettlementRepository) Pending(ctx context.Context, limit int) ([]domain.SettlementLeg, error) {
	rows, err := r.Pool.Query(ctx,
		`SELECT id, listing_id, escrow_ref, recipient_account_id, deliver_amount, whole
		 FROM cp_settlement WHERE status = 1 ORDER BY id LIMIT $1`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("infra.SettlementRepository.Pending: %w", err)
	}
	defer rows.Close()

	out := make([]domain.SettlementLeg, 0)
	for rows.Next() {
		var leg domain.SettlementLeg
		if err := rows.Scan(&leg.ID, &leg.ListingID, &leg.EscrowRef, &leg.RecipientAccountID, &leg.DeliverAmount, &leg.Whole); err != nil {
			return nil, fmt.Errorf("infra.SettlementRepository.Pending scan: %w", err)
		}
		out = append(out, leg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("infra.SettlementRepository.Pending rows: %w", err)
	}

	return out, nil
}

func (r *SettlementRepository) PendingRefs(ctx context.Context) ([]int64, error) {
	rows, err := r.Pool.Query(ctx, `SELECT DISTINCT escrow_ref FROM cp_settlement WHERE status = 1`)
	if err != nil {
		return nil, fmt.Errorf("infra.SettlementRepository.PendingRefs: %w", err)
	}
	defer rows.Close()

	out := make([]int64, 0)
	for rows.Next() {
		var ref int64
		if err := rows.Scan(&ref); err != nil {
			return nil, fmt.Errorf("infra.SettlementRepository.PendingRefs scan: %w", err)
		}
		out = append(out, ref)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("infra.SettlementRepository.PendingRefs rows: %w", err)
	}

	return out, nil
}

func (r *SettlementRepository) MarkDone(ctx context.Context, id int64) error {
	if _, err := r.Pool.Exec(ctx, `UPDATE cp_settlement SET status = 2 WHERE id = $1`, id); err != nil {
		return fmt.Errorf("infra.SettlementRepository.MarkDone: %w", err)
	}

	return nil
}
