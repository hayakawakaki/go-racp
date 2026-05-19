package infra

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hayakawakaki/go-racp/internal/features/character/domain"
	"github.com/hayakawakaki/go-racp/internal/infra/postgres"
)

type CooldownRepository = postgres.RecordStore[domain.ChangeType]

func NewCooldownRepository(pool *pgxpool.Pool) *CooldownRepository {
	return postgres.NewRecordStore[domain.ChangeType](pool, "cp_character_record", "char_id")
}
