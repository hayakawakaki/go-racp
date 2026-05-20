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

func (s *MariaSource) CountOnlineTotal(ctx context.Context) (int, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM `char` WHERE online = 1").Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("infra.MariaSource.CountOnlineTotal: %w", err)
	}
	return n, nil
}

func (s *MariaSource) CountVendors(ctx context.Context) (int, error) {
	var vendings, buying int
	if err := s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM vendings").Scan(&vendings); err != nil {
		return 0, fmt.Errorf("infra.MariaSource.CountVendors vendings: %w", err)
	}
	if err := s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM buyingstores").Scan(&buying); err != nil {
		return 0, fmt.Errorf("infra.MariaSource.CountVendors buyingstores: %w", err)
	}
	return vendings + buying, nil
}

func (s *MariaSource) CountUniqueOnline(ctx context.Context) (int, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, "SELECT COUNT(DISTINCT l.last_unique_id) FROM login l JOIN `char` c ON c.account_id = l.account_id WHERE c.online = 1").Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("infra.MariaSource.CountUniqueOnline: %w", err)
	}
	return n, nil
}

func (s *MariaSource) CountAccounts(ctx context.Context) (int, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM login").Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("infra.MariaSource.CountAccounts: %w", err)
	}
	return n, nil
}

func (s *MariaSource) CountCharacters(ctx context.Context) (int, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM `char`").Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("infra.MariaSource.CountCharacters: %w", err)
	}
	return n, nil
}

func (s *MariaSource) CountGuilds(ctx context.Context) (int, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM guild").Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("infra.MariaSource.CountGuilds: %w", err)
	}
	return n, nil
}
