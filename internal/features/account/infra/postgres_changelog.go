package infra

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
	"github.com/hayakawakaki/go-racp/internal/infra/postgres"
)

type ChangeLogRepository = postgres.RecordStore[domain.ChangeType]

func NewChangeLogRepository(pool *pgxpool.Pool) *ChangeLogRepository {
	return postgres.NewRecordStore[domain.ChangeType](pool, "cp_account_record", "account_id")
}
