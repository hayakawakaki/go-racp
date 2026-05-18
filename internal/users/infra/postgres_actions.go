package infra

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hayakawakaki/go-racp/internal/users/domain"
)

type ActionRepository struct {
	Pool *pgxpool.Pool
}

func NewActionRepository(pool *pgxpool.Pool) *ActionRepository {
	return &ActionRepository{Pool: pool}
}

func (r *ActionRepository) Record(ctx context.Context, a domain.Action) error {
	_, err := r.Pool.Exec(ctx,
		`INSERT INTO cp_user_actions (actor_user_id, target_user_id, action, reason, before_value, after_value)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		a.ActorUserID, a.TargetUserID, string(a.Kind), a.Reason, a.BeforeValue, a.AfterValue,
	)
	if err != nil {
		return fmt.Errorf("infra.ActionRepository.Record: %w", err)
	}

	return nil
}

func (r *ActionRepository) ListByTarget(ctx context.Context, targetID, limit int) ([]domain.Action, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.Pool.Query(ctx,
		`SELECT id, actor_user_id, target_user_id, action, reason, before_value, after_value, created_at
		 FROM cp_user_actions WHERE target_user_id = $1
		 ORDER BY created_at DESC, id DESC LIMIT $2`,
		targetID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("infra.ActionRepository.ListByTarget: %w", err)
	}
	defer rows.Close()

	out := make([]domain.Action, 0, limit)
	for rows.Next() {
		var a domain.Action
		var kind string
		if err := rows.Scan(&a.ID, &a.ActorUserID, &a.TargetUserID, &kind, &a.Reason, &a.BeforeValue, &a.AfterValue, &a.At); err != nil {
			return nil, fmt.Errorf("infra.ActionRepository.ListByTarget scan: %w", err)
		}
		a.Kind = domain.ActionKind(kind)
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("infra.ActionRepository.ListByTarget rows: %w", err)
	}

	return out, nil
}
