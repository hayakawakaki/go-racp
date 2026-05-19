package infra

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/hayakawakaki/go-racp/internal/features/users/domain"
)

type CharRepository struct {
	Client *sql.DB
}

func NewCharRepository(client *sql.DB) *CharRepository {
	return &CharRepository{Client: client}
}

func (r *CharRepository) ListByAccount(ctx context.Context, accountID int) ([]domain.Character, error) {
	rows, err := r.Client.QueryContext(ctx,
		"SELECT char_id, account_id, name, class, base_level, job_level, zeny, last_map, online, last_login "+
			"FROM `char` WHERE account_id = ? ORDER BY char_num ASC",
		accountID,
	)
	if err != nil {
		return nil, fmt.Errorf("infra.CharRepository.ListByAccount: %w", err)
	}
	defer func() { _ = rows.Close() }()

	chars := make([]domain.Character, 0)
	for rows.Next() {
		var (
			c         domain.Character
			lastMap   sql.NullString
			lastLogin sql.NullTime
			onlineRaw int
		)
		if err := rows.Scan(&c.ID, &c.AccountID, &c.Name, &c.Class, &c.BaseLevel, &c.JobLevel, &c.Zeny, &lastMap, &onlineRaw, &lastLogin); err != nil {
			return nil, fmt.Errorf("infra.CharRepository.ListByAccount scan: %w", err)
		}
		c.LastMap = lastMap.String
		c.Online = onlineRaw != 0
		if lastLogin.Valid {
			c.LastLogin = lastLogin.Time
		}
		chars = append(chars, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("infra.CharRepository.ListByAccount rows: %w", err)
	}

	return chars, nil
}
