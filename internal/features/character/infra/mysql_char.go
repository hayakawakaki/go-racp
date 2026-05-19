package infra

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	domain2 "github.com/hayakawakaki/go-racp/internal/features/character/domain"
)

const charSelectColumns = "char_id, account_id, char_num, name, zeny, sex, class, base_level, job_level, " +
	"hair, hair_color, clothes_color, body, last_map, last_x, last_y, save_map, save_x, save_y, " +
	"head_top, head_mid, head_bottom, robe, online"

type Repository struct {
	Client *sql.DB
}

func NewRepository(client *sql.DB) *Repository {
	return &Repository{Client: client}
}

func (r *Repository) ListByAccount(ctx context.Context, accountID int) ([]domain2.Character, error) {
	rows, err := r.Client.QueryContext(ctx,
		"SELECT "+charSelectColumns+" FROM `char` WHERE account_id = ? ORDER BY char_num",
		accountID,
	)
	if err != nil {
		return nil, fmt.Errorf("infra.Repository.ListByAccount: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []domain2.Character
	for rows.Next() {
		c, err := scanCharacter(rows)
		if err != nil {
			return nil, fmt.Errorf("infra.Repository.ListByAccount: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("infra.Repository.ListByAccount: %w", err)
	}

	return out, nil
}

func (r *Repository) GetByID(ctx context.Context, charID int) (*domain2.Character, error) {
	row := r.Client.QueryRowContext(ctx,
		"SELECT "+charSelectColumns+" FROM `char` WHERE char_id = ?",
		charID,
	)
	c, err := scanCharacter(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain2.ErrCharacterNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("infra.Repository.GetByID: %w", err)
	}

	return &c, nil
}

func (r *Repository) UpdateLook(ctx context.Context, charID, hair, hairColor, clothesColor int) error {
	res, err := r.Client.ExecContext(ctx,
		"UPDATE `char` SET hair = ?, hair_color = ?, clothes_color = ? WHERE char_id = ?",
		hair, hairColor, clothesColor, charID,
	)
	if err != nil {
		return fmt.Errorf("infra.Repository.UpdateLook: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("infra.Repository.UpdateLook: %w", err)
	}
	if rows == 0 {
		return domain2.ErrCharacterNotFound
	}

	return nil
}

func (r *Repository) UpdateLocation(ctx context.Context, charID int, mapName string, x, y int) error {
	res, err := r.Client.ExecContext(ctx,
		"UPDATE `char` SET last_map = ?, last_x = ?, last_y = ? WHERE char_id = ?",
		mapName, x, y, charID,
	)
	if err != nil {
		return fmt.Errorf("infra.Repository.UpdateLocation: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("infra.Repository.UpdateLocation: %w", err)
	}
	if rows == 0 {
		return domain2.ErrCharacterNotFound
	}

	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanCharacter(s rowScanner) (domain2.Character, error) {
	var c domain2.Character
	var online int
	err := s.Scan(
		&c.ID, &c.AccountID, &c.Slot, &c.Name, &c.Zeny, &c.Gender, &c.JobID,
		&c.BaseLevel, &c.JobLevel,
		&c.HairStyle, &c.HairColor, &c.ClothesColor, &c.BodyID,
		&c.CurrentMap, &c.CurrentX, &c.CurrentY,
		&c.SaveMap, &c.SaveX, &c.SaveY,
		&c.CostumeTop, &c.CostumeMid, &c.CostumeBottom, &c.CostumeRobe,
		&online,
	)
	if err != nil {
		return domain2.Character{}, fmt.Errorf("scan character: %w", err)
	}
	c.Online = online != 0

	return c, nil
}
