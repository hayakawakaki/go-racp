package infra

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hayakawakaki/go-racp/internal/platform/notification/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const notificationColumns = `id, account_id, category, title, body, link, read_at, created_at`

type Repository struct {
	Pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{Pool: pool}
}

func (r *Repository) Create(ctx context.Context, notification domain.Notification) (domain.Notification, error) {
	row := r.Pool.QueryRow(ctx,
		`INSERT INTO cp_notification (account_id, category, title, body, link)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING `+notificationColumns,
		notification.AccountID, notification.Category, notification.Title, notification.Body, notification.Link,
	)

	created, err := scanNotification(row)
	if err != nil {
		return domain.Notification{}, fmt.Errorf("infra.Repository.Create: %w", err)
	}

	return created, nil
}

func (r *Repository) RecentByAccount(ctx context.Context, accountID, limit int) ([]domain.Notification, error) {
	rows, err := r.Pool.Query(ctx,
		`SELECT `+notificationColumns+`
		 FROM cp_notification
		 WHERE account_id = $1
		 ORDER BY created_at DESC, id DESC
		 LIMIT $2`,
		accountID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("infra.Repository.RecentByAccount: %w", err)
	}
	defer rows.Close()

	out, err := collectNotifications(rows)
	if err != nil {
		return nil, fmt.Errorf("infra.Repository.RecentByAccount: %w", err)
	}

	return out, nil
}

func (r *Repository) ListPage(ctx context.Context, accountID int, unreadOnly bool, limit, offset int) ([]domain.Notification, int, error) {
	countQuery := `SELECT COUNT(*) FROM cp_notification WHERE account_id = $1`
	listQuery := `SELECT ` + notificationColumns + `
		 FROM cp_notification
		 WHERE account_id = $1
		 ORDER BY created_at DESC, id DESC
		 LIMIT $2 OFFSET $3`
	if unreadOnly {
		countQuery = `SELECT COUNT(*) FROM cp_notification WHERE account_id = $1 AND read_at IS NULL`
		listQuery = `SELECT ` + notificationColumns + `
		 FROM cp_notification
		 WHERE account_id = $1 AND read_at IS NULL
		 ORDER BY created_at DESC, id DESC
		 LIMIT $2 OFFSET $3`
	}

	var total int
	if err := r.Pool.QueryRow(ctx, countQuery, accountID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("infra.Repository.ListPage: %w", err)
	}

	rows, err := r.Pool.Query(ctx, listQuery, accountID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("infra.Repository.ListPage: %w", err)
	}
	defer rows.Close()

	out, err := collectNotifications(rows)
	if err != nil {
		return nil, 0, fmt.Errorf("infra.Repository.ListPage: %w", err)
	}

	return out, total, nil
}

func (r *Repository) UnreadCount(ctx context.Context, accountID int) (int, error) {
	var count int
	err := r.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM cp_notification WHERE account_id = $1 AND read_at IS NULL`,
		accountID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("infra.Repository.UnreadCount: %w", err)
	}

	return count, nil
}

func (r *Repository) MarkRead(ctx context.Context, accountID int, id int64, now time.Time) (string, error) {
	var link string
	err := r.Pool.QueryRow(ctx,
		`UPDATE cp_notification SET read_at = COALESCE(read_at, $1)
		 WHERE id = $2 AND account_id = $3
		 RETURNING link`,
		now, id, accountID,
	).Scan(&link)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("infra.Repository.MarkRead: %w", err)
	}

	return link, nil
}

func (r *Repository) MarkAllRead(ctx context.Context, accountID int, now time.Time) (int64, error) {
	tag, err := r.Pool.Exec(ctx,
		`UPDATE cp_notification SET read_at = $1
		 WHERE account_id = $2 AND read_at IS NULL`,
		now, accountID,
	)
	if err != nil {
		return 0, fmt.Errorf("infra.Repository.MarkAllRead: %w", err)
	}

	return tag.RowsAffected(), nil
}

func (r *Repository) PruneOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	tag, err := r.Pool.Exec(ctx,
		`DELETE FROM cp_notification WHERE created_at < $1`,
		cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("infra.Repository.PruneOlderThan: %w", err)
	}

	return tag.RowsAffected(), nil
}

func scanNotification(row pgx.Row) (domain.Notification, error) {
	var n domain.Notification
	if err := row.Scan(&n.ID, &n.AccountID, &n.Category, &n.Title, &n.Body, &n.Link, &n.ReadAt, &n.CreatedAt); err != nil {
		return domain.Notification{}, fmt.Errorf("infra.scanNotification: %w", err)
	}

	return n, nil
}

func collectNotifications(rows pgx.Rows) ([]domain.Notification, error) {
	out := make([]domain.Notification, 0)
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("infra.collectNotifications: %w", err)
	}

	return out, nil
}
