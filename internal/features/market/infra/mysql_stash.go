package infra

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/hayakawakaki/go-racp/internal/features/market/domain"
)

const itemColumns = `id, account_id, nameid, amount, equip, identify, refine, attribute,
	card0, card1, card2, card3,
	option_id0, option_val0, option_parm0,
	option_id1, option_val1, option_parm1,
	option_id2, option_val2, option_parm2,
	option_id3, option_val3, option_parm3,
	option_id4, option_val4, option_parm4,
	expire_time, bound, unique_id, enchantgrade`

const stackableMatchWhere = `account_id = ? AND nameid = ? AND identify = ? AND expire_time = ?
	AND refine = 0 AND enchantgrade = 0 AND bound = 0 AND equip = 0 AND attribute = 0 AND unique_id = 0
	AND card0 = 0 AND card1 = 0 AND card2 = 0 AND card3 = 0
	AND option_id0 = 0 AND option_val0 = 0 AND option_parm0 = 0
	AND option_id1 = 0 AND option_val1 = 0 AND option_parm1 = 0
	AND option_id2 = 0 AND option_val2 = 0 AND option_parm2 = 0
	AND option_id3 = 0 AND option_val3 = 0 AND option_parm3 = 0
	AND option_id4 = 0 AND option_val4 = 0 AND option_parm4 = 0`

type StashRepository struct {
	Client *sql.DB
}

func NewStashRepository(client *sql.DB) *StashRepository {
	return &StashRepository{Client: client}
}

func scanStashItem(rows *sql.Rows) (domain.StashItem, error) {
	var item domain.StashItem
	err := rows.Scan(
		&item.ID, &item.AccountID, &item.NameID, &item.Amount, &item.Equip, &item.Identify, &item.Refine, &item.Attribute,
		&item.Card[0], &item.Card[1], &item.Card[2], &item.Card[3],
		&item.OptionID[0], &item.OptionVal[0], &item.OptionParm[0],
		&item.OptionID[1], &item.OptionVal[1], &item.OptionParm[1],
		&item.OptionID[2], &item.OptionVal[2], &item.OptionParm[2],
		&item.OptionID[3], &item.OptionVal[3], &item.OptionParm[3],
		&item.OptionID[4], &item.OptionVal[4], &item.OptionParm[4],
		&item.ExpireTime, &item.Bound, &item.UniqueID, &item.Grade,
	)
	if err != nil {
		return domain.StashItem{}, fmt.Errorf("infra.scanStashItem: %w", err)
	}

	return item, nil
}

func (r *StashRepository) ListByAccount(ctx context.Context, accountID int) ([]domain.StashItem, error) {
	rows, err := r.Client.QueryContext(ctx,
		"SELECT "+itemColumns+" FROM cp_storage WHERE account_id = ? ORDER BY id", accountID,
	)
	if err != nil {
		return nil, fmt.Errorf("infra.StashRepository.ListByAccount: %w", err)
	}

	items, err := collectRows(rows, scanStashItem)
	if err != nil {
		return nil, fmt.Errorf("infra.StashRepository.ListByAccount: %w", err)
	}

	return items, nil
}

func (r *StashRepository) IsLocked(ctx context.Context, accountID int) (bool, error) {
	var locked bool
	err := r.Client.QueryRowContext(ctx,
		"SELECT is_locked FROM cp_storage_lock WHERE account_id = ?", accountID,
	).Scan(&locked)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("infra.StashRepository.IsLocked: %w", err)
	}

	return locked, nil
}

func (r *StashRepository) SlotsUsed(ctx context.Context, accountID int) (int, error) {
	var used int
	err := r.Client.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM cp_storage WHERE account_id = ?", accountID,
	).Scan(&used)
	if err != nil {
		return 0, fmt.Errorf("infra.StashRepository.SlotsUsed: %w", err)
	}

	return used, nil
}

func (r *StashRepository) MergeableAmount(ctx context.Context, accountID int, item domain.StashItem) (existingAmount int, found bool, err error) {
	if !item.IsStackable() {
		return 0, false, nil
	}

	err = r.Client.QueryRowContext(ctx,
		"SELECT amount FROM cp_storage WHERE "+stackableMatchWhere+" LIMIT 1",
		accountID, item.NameID, item.Identify, item.ExpireTime,
	).Scan(&existingAmount)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("infra.StashRepository.MergeableAmount: %w", err)
	}

	return existingAmount, true, nil
}
