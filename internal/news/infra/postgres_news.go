package infra

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hayakawakaki/go-racp/internal/news/domain"
)

const newsColumns = `id, title, body, category, created_at`

type Repository struct {
	Pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{Pool: pool}
}

func (r *Repository) Create(ctx context.Context, news domain.News) (int64, error) {
	var id int64
	err := r.Pool.QueryRow(ctx,
		`INSERT INTO cp_news (title, body, category) VALUES ($1, $2, $3) RETURNING id`,
		news.Title, news.Body, news.Category,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("infra.Repository.Create: %w", err)
	}

	return id, nil
}

func (r *Repository) Update(ctx context.Context, news domain.News) error {
	tag, err := r.Pool.Exec(ctx,
		`UPDATE cp_news SET title = $1, body = $2, category = $3 WHERE id = $4`,
		news.Title, news.Body, news.Category, news.ID,
	)
	if err != nil {
		return fmt.Errorf("infra.Repository.Update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	tag, err := r.Pool.Exec(ctx, `DELETE FROM cp_news WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("infra.Repository.Delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func (r *Repository) Get(ctx context.Context, id int64) (domain.News, error) {
	row := r.Pool.QueryRow(ctx,
		`SELECT `+newsColumns+` FROM cp_news WHERE id = $1`, id,
	)
	news, err := scanNews(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.News{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.News{}, fmt.Errorf("infra.Repository.Get: %w", err)
	}

	return news, nil
}

func (r *Repository) List(ctx context.Context) ([]domain.News, error) {
	rows, err := r.Pool.Query(ctx,
		`SELECT `+newsColumns+` FROM cp_news ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("infra.Repository.List: %w", err)
	}
	defer rows.Close()

	out, err := collectNews(rows)
	if err != nil {
		return nil, fmt.Errorf("infra.Repository.List: %w", err)
	}

	return out, nil
}

func (r *Repository) ListByCategory(ctx context.Context, category string) ([]domain.News, error) {
	rows, err := r.Pool.Query(ctx,
		`SELECT `+newsColumns+` FROM cp_news WHERE category = $1 ORDER BY created_at DESC`,
		category,
	)
	if err != nil {
		return nil, fmt.Errorf("infra.Repository.ListByCategory: %w", err)
	}
	defer rows.Close()

	out, err := collectNews(rows)
	if err != nil {
		return nil, fmt.Errorf("infra.Repository.ListByCategory: %w", err)
	}

	return out, nil
}

func scanNews(row pgx.Row) (domain.News, error) {
	var n domain.News
	if err := row.Scan(&n.ID, &n.Title, &n.Body, &n.Category, &n.CreatedAt); err != nil {
		return domain.News{}, fmt.Errorf("infra.scanNews: %w", err)
	}

	return n, nil
}

func collectNews(rows pgx.Rows) ([]domain.News, error) {
	out := make([]domain.News, 0)
	for rows.Next() {
		n, err := scanNews(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("infra.collectNews: %w", err)
	}

	return out, nil
}
