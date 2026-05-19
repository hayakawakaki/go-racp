package infra

import (
	"context"
	"fmt"

	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuditRepository struct {
	Pool *pgxpool.Pool
}

func NewAuditRepository(pool *pgxpool.Pool) *AuditRepository {
	return &AuditRepository{Pool: pool}
}

func (r *AuditRepository) Record(ctx context.Context, a domain.AuditEntry) error {
	_, err := r.Pool.Exec(ctx,
		`INSERT INTO cp_audit_log (actor_user_id, target_user_id, action, reason, before_value, after_value)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		a.ActorUserID, a.TargetUserID, string(a.Kind), a.Reason, a.BeforeValue, a.AfterValue,
	)
	if err != nil {
		return fmt.Errorf("infra.AuditRepository.Record: %w", err)
	}

	return nil
}

func (r *AuditRepository) ListByTarget(ctx context.Context, targetID, limit int) ([]domain.AuditEntry, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.Pool.Query(ctx,
		`SELECT id, actor_user_id, target_user_id, action, reason, before_value, after_value, created_at
		 FROM cp_audit_log WHERE target_user_id = $1
		 ORDER BY created_at DESC, id DESC LIMIT $2`,
		targetID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("infra.AuditRepository.ListByTarget: %w", err)
	}
	defer rows.Close()

	out := make([]domain.AuditEntry, 0, limit)
	for rows.Next() {
		var a domain.AuditEntry
		var kind string
		if err := rows.Scan(&a.ID, &a.ActorUserID, &a.TargetUserID, &kind, &a.Reason, &a.BeforeValue, &a.AfterValue, &a.At); err != nil {
			return nil, fmt.Errorf("infra.AuditRepository.ListByTarget scan: %w", err)
		}
		a.Kind = domain.AuditKind(kind)
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("infra.AuditRepository.ListByTarget rows: %w", err)
	}

	return out, nil
}
