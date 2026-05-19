package infra

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/hayakawakaki/go-racp/internal/features/guild/domain"
)

const (
	DefaultPerPage = 20
	MaxPerPage     = 100
)

const guildSelectColumns = "guild_id, name, master, char_id, guild_lv, max_member"

type Repository struct {
	Client *sql.DB
}

func NewRepository(client *sql.DB) *Repository {
	return &Repository{Client: client}
}

func (r *Repository) GetByID(ctx context.Context, id int) (*domain.Guild, error) {
	var g domain.Guild
	err := r.Client.QueryRowContext(ctx,
		"SELECT "+guildSelectColumns+" FROM guild WHERE guild_id = ?", id,
	).Scan(&g.ID, &g.Name, &g.MasterName, &g.MasterCharID, &g.GuildLevel, &g.MaxMember)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrGuildNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("infra.Repository.GetByID: %w", err)
	}

	return &g, nil
}

func (r *Repository) List(ctx context.Context, q domain.ListQuery) (domain.GuildPage, error) {
	if q.PerPage <= 0 {
		q.PerPage = DefaultPerPage
	}
	if q.PerPage > MaxPerPage {
		q.PerPage = MaxPerPage
	}
	if q.Page <= 0 {
		q.Page = 1
	}

	where, args := buildWhere(q)

	var total int
	if err := r.Client.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM guild "+where, args...,
	).Scan(&total); err != nil {
		return domain.GuildPage{}, fmt.Errorf("infra.Repository.List count: %w", err)
	}

	offset := (q.Page - 1) * q.PerPage
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, q.PerPage, offset)

	rows, err := r.Client.QueryContext(ctx,
		"SELECT "+guildSelectColumns+" FROM guild "+where+ //nolint:gosec // where built from constants
			" ORDER BY guild_id ASC LIMIT ? OFFSET ?", queryArgs...,
	)
	if err != nil {
		return domain.GuildPage{}, fmt.Errorf("infra.Repository.List query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	guilds := make([]domain.Guild, 0, q.PerPage)
	for rows.Next() {
		var g domain.Guild
		if err := rows.Scan(&g.ID, &g.Name, &g.MasterName, &g.MasterCharID, &g.GuildLevel, &g.MaxMember); err != nil {
			return domain.GuildPage{}, fmt.Errorf("infra.Repository.List scan: %w", err)
		}
		guilds = append(guilds, g)
	}
	if err := rows.Err(); err != nil {
		return domain.GuildPage{}, fmt.Errorf("infra.Repository.List rows: %w", err)
	}

	totalPages := (total + q.PerPage - 1) / q.PerPage

	return domain.GuildPage{
		Guilds:     guilds,
		Total:      total,
		Page:       q.Page,
		PerPage:    q.PerPage,
		TotalPages: totalPages,
	}, nil
}

func (r *Repository) ListMembers(ctx context.Context, guildID int) ([]domain.Member, error) {
	rows, err := r.Client.QueryContext(ctx,
		"SELECT gm.char_id, gm.name, gm.position, gp.name "+
			"FROM guild_member gm "+
			"LEFT JOIN guild_position gp ON gp.guild_id = gm.guild_id AND gp.position = gm.position "+
			"WHERE gm.guild_id = ? "+
			"ORDER BY gm.position ASC, gm.name ASC",
		guildID,
	)
	if err != nil {
		return nil, fmt.Errorf("infra.Repository.ListMembers: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []domain.Member
	for rows.Next() {
		var (
			m       domain.Member
			posName sql.NullString
		)
		if err := rows.Scan(&m.CharID, &m.Name, &m.Position, &posName); err != nil {
			return nil, fmt.Errorf("infra.Repository.ListMembers scan: %w", err)
		}
		m.PositionName = posName.String
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("infra.Repository.ListMembers rows: %w", err)
	}

	return out, nil
}

func (r *Repository) GetEmblem(ctx context.Context, guildID int) (data []byte, mime string, err error) {
	var size int
	err = r.Client.QueryRowContext(ctx,
		"SELECT emblem_data, emblem_len FROM guild WHERE guild_id = ?", guildID,
	).Scan(&data, &size)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", domain.ErrGuildNotFound
	}
	if err != nil {
		return nil, "", fmt.Errorf("infra.Repository.GetEmblem: %w", err)
	}
	if size == 0 || len(data) == 0 {
		return nil, "", domain.ErrEmblemEmpty
	}

	switch {
	case bytes.HasPrefix(data, []byte{'B', 'M'}):
		return data, "image/bmp", nil
	case bytes.HasPrefix(data, []byte("GIF8")):
		return data, "image/gif", nil
	}

	return nil, "", domain.ErrEmblemUnknownFormat
}

func buildWhere(q domain.ListQuery) (where string, args []any) {
	clauses := make([]string, 0, 1)
	args = make([]any, 0, 1)
	if needle := strings.TrimSpace(q.Query); needle != "" {
		clauses = append(clauses, "name LIKE ?")
		args = append(args, "%"+needle+"%")
	}
	if len(clauses) == 0 {
		return "", args
	}

	return "WHERE " + strings.Join(clauses, " AND "), args
}
