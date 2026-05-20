package infra

import (
	"context"
	"database/sql"
	"fmt"
)

type MariaSource struct {
	DB *sql.DB
}

func NewMariaSource(db *sql.DB) *MariaSource {
	return &MariaSource{DB: db}
}

func (s *MariaSource) countOne(ctx context.Context, label, query string) (int, error) {
	var n int
	if err := s.DB.QueryRowContext(ctx, query).Scan(&n); err != nil {
		return 0, fmt.Errorf("infra.MariaSource.%s: %w", label, err)
	}
	return n, nil
}

func (s *MariaSource) CountOnlineTotal(ctx context.Context) (int, error) {
	return s.countOne(ctx, "CountOnlineTotal",
		"SELECT COUNT(*) FROM `char` WHERE online = 1")
}

func (s *MariaSource) CountVendors(ctx context.Context) (int, error) {
	return s.countOne(ctx, "CountVendors",
		"SELECT (SELECT COUNT(*) FROM vendings) + (SELECT COUNT(*) FROM buyingstores)")
}

func (s *MariaSource) CountUniqueOnline(ctx context.Context) (int, error) {
	return s.countOne(ctx, "CountUniqueOnline",
		"SELECT COUNT(DISTINCT l.last_unique_id) FROM login l JOIN `char` c ON c.account_id = l.account_id WHERE c.online = 1")
}

func (s *MariaSource) CountAccounts(ctx context.Context) (int, error) {
	return s.countOne(ctx, "CountAccounts", "SELECT COUNT(*) FROM login")
}

func (s *MariaSource) CountCharacters(ctx context.Context) (int, error) {
	return s.countOne(ctx, "CountCharacters", "SELECT COUNT(*) FROM `char`")
}

func (s *MariaSource) CountGuilds(ctx context.Context) (int, error) {
	return s.countOne(ctx, "CountGuilds", "SELECT COUNT(*) FROM guild")
}
