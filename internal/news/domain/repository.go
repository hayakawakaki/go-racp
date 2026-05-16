package domain

import "context"

type Repository interface {
	Create(ctx context.Context, news News) (int64, error)
	Update(ctx context.Context, news News) error
	Delete(ctx context.Context, id int64) error
	Get(ctx context.Context, id int64) (News, error)
	List(ctx context.Context) ([]News, error)
	ListByCategory(ctx context.Context, category string) ([]News, error)
}
