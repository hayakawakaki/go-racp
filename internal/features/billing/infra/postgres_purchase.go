package infra

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/billing/app"
	"github.com/hayakawakaki/go-racp/internal/features/billing/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ app.Repository = (*PurchaseRepository)(nil)

const purchaseColumns = `id, account_id, package_key, provider, COALESCE(provider_ref, ''), COALESCE(provider_payment_id, ''), amount, currency, cash_points, status, created_at, completed_at, disputed_at`

type PurchaseRepository struct {
	Pool *pgxpool.Pool
}

func NewPurchaseRepository(pool *pgxpool.Pool) *PurchaseRepository {
	return &PurchaseRepository{Pool: pool}
}

func (r *PurchaseRepository) Create(ctx context.Context, purchase domain.Purchase) (int64, error) {
	var id int64
	if err := r.Pool.QueryRow(ctx,
		`INSERT INTO cp_purchases (account_id, package_key, provider, amount, currency, cash_points, status, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
		purchase.AccountID, purchase.PackageKey, purchase.Provider, purchase.Amount, purchase.Currency, purchase.CashPoints, purchase.Status, purchase.CreatedAt,
	).Scan(&id); err != nil {
		return 0, fmt.Errorf("infra.PurchaseRepository.Create: %w", err)
	}

	return id, nil
}

func (r *PurchaseRepository) SetProviderRef(ctx context.Context, id int64, ref string) error {
	if _, err := r.Pool.Exec(ctx,
		`UPDATE cp_purchases SET provider_ref = $1 WHERE id = $2`, ref, id,
	); err != nil {
		return fmt.Errorf("infra.PurchaseRepository.SetProviderRef: %w", err)
	}

	return nil
}

func (r *PurchaseRepository) GetByID(ctx context.Context, id int64) (domain.Purchase, error) {
	row := r.Pool.QueryRow(ctx,
		`SELECT `+purchaseColumns+` FROM cp_purchases WHERE id = $1`, id,
	)

	purchase, err := scanPurchase(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Purchase{}, domain.ErrPurchaseNotFound
	}
	if err != nil {
		return domain.Purchase{}, fmt.Errorf("infra.PurchaseRepository.GetByID: %w", err)
	}

	return purchase, nil
}

func (r *PurchaseRepository) GetByPaymentID(ctx context.Context, provider, paymentID string) (domain.Purchase, error) {
	row := r.Pool.QueryRow(ctx,
		`SELECT `+purchaseColumns+` FROM cp_purchases WHERE provider = $1 AND provider_payment_id = $2`,
		provider, paymentID,
	)

	purchase, err := scanPurchase(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Purchase{}, domain.ErrPurchaseNotFound
	}
	if err != nil {
		return domain.Purchase{}, fmt.Errorf("infra.PurchaseRepository.GetByPaymentID: %w", err)
	}

	return purchase, nil
}

func (r *PurchaseRepository) Complete(ctx context.Context, id int64, providerPaymentID string, now time.Time) (credited bool, accountID, cashPoints int, err error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return false, 0, 0, fmt.Errorf("infra.PurchaseRepository.Complete begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	err = tx.QueryRow(ctx,
		`UPDATE cp_purchases SET status = 2, provider_payment_id = $1, completed_at = $2
		 WHERE id = $3 AND status = 1
		 RETURNING account_id, cash_points`,
		providerPaymentID, now, id,
	).Scan(&accountID, &cashPoints)
	if errors.Is(err, pgx.ErrNoRows) {
		if err = tx.Commit(ctx); err != nil {
			return false, 0, 0, fmt.Errorf("infra.PurchaseRepository.Complete commit: %w", err)
		}
		return false, 0, 0, nil
	}
	if err != nil {
		return false, 0, 0, fmt.Errorf("infra.PurchaseRepository.Complete mark: %w", err)
	}

	if _, err = tx.Exec(ctx,
		`INSERT INTO cp_currency (account_id) VALUES ($1) ON CONFLICT (account_id) DO NOTHING`, accountID,
	); err != nil {
		return false, 0, 0, fmt.Errorf("infra.PurchaseRepository.Complete ensure wallet: %w", err)
	}
	if _, err = tx.Exec(ctx,
		`UPDATE cp_currency SET cashpoint = cashpoint + $1 WHERE account_id = $2`, cashPoints, accountID,
	); err != nil {
		return false, 0, 0, fmt.Errorf("infra.PurchaseRepository.Complete credit: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return false, 0, 0, fmt.Errorf("infra.PurchaseRepository.Complete commit: %w", err)
	}

	return true, accountID, cashPoints, nil
}

func (r *PurchaseRepository) MarkDisputed(ctx context.Context, id int64, now time.Time) (bool, error) {
	tag, err := r.Pool.Exec(ctx,
		`UPDATE cp_purchases SET status = 3, disputed_at = $1 WHERE id = $2 AND status = 2`,
		now, id,
	)
	if err != nil {
		return false, fmt.Errorf("infra.PurchaseRepository.MarkDisputed: %w", err)
	}

	return tag.RowsAffected() > 0, nil
}

func (r *PurchaseRepository) MarkRefunded(ctx context.Context, id int64, now time.Time) (bool, error) {
	tag, err := r.Pool.Exec(ctx,
		`UPDATE cp_purchases SET status = 4, refunded_at = $1 WHERE id = $2 AND status = 2`,
		now, id,
	)
	if err != nil {
		return false, fmt.Errorf("infra.PurchaseRepository.MarkRefunded: %w", err)
	}

	return tag.RowsAffected() > 0, nil
}

func (r *PurchaseRepository) MarkFailed(ctx context.Context, id int64, now time.Time) (bool, error) {
	tag, err := r.Pool.Exec(ctx,
		`UPDATE cp_purchases SET status = 5, failed_at = $1 WHERE id = $2 AND status = 1`,
		now, id,
	)
	if err != nil {
		return false, fmt.Errorf("infra.PurchaseRepository.MarkFailed: %w", err)
	}

	return tag.RowsAffected() > 0, nil
}

func (r *PurchaseRepository) ListPaidByAccount(ctx context.Context, accountID, limit int) ([]domain.Purchase, error) {
	rows, err := r.Pool.Query(ctx,
		`SELECT `+purchaseColumns+` FROM cp_purchases WHERE account_id = $1 AND status IN (2, 3, 4) ORDER BY id DESC LIMIT $2`,
		accountID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("infra.PurchaseRepository.ListPaidByAccount: %w", err)
	}

	return scanPurchases(rows)
}

func buildPurchaseFilter(filter domain.PurchaseFilter) (whereClause string, args []any) {
	var predicates []string
	if filter.Status != 0 {
		args = append(args, filter.Status)
		predicates = append(predicates, "status = $"+strconv.Itoa(len(args)))
	}
	if filter.AccountID != 0 {
		args = append(args, filter.AccountID)
		predicates = append(predicates, "account_id = $"+strconv.Itoa(len(args)))
	}
	if filter.Provider != "" {
		args = append(args, filter.Provider)
		predicates = append(predicates, "provider = $"+strconv.Itoa(len(args)))
	}
	if filter.From != nil {
		args = append(args, *filter.From)
		predicates = append(predicates, "created_at >= $"+strconv.Itoa(len(args)))
	}
	if filter.To != nil {
		args = append(args, *filter.To)
		predicates = append(predicates, "created_at < $"+strconv.Itoa(len(args)))
	}
	if len(predicates) == 0 {
		return "", args
	}

	return " WHERE " + strings.Join(predicates, " AND "), args
}

func (r *PurchaseRepository) ListFiltered(ctx context.Context, filter domain.PurchaseFilter, limit, offset int) (rows []domain.Purchase, total int, err error) {
	whereClause, args := buildPurchaseFilter(filter)

	if err = r.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM cp_purchases`+whereClause, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("infra.PurchaseRepository.ListFiltered: %w", err)
	}

	args = append(args, limit, offset)
	query := `SELECT ` + purchaseColumns + ` FROM cp_purchases` + whereClause +
		` ORDER BY id DESC LIMIT $` + strconv.Itoa(len(args)-1) + ` OFFSET $` + strconv.Itoa(len(args))

	queried, err := r.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("infra.PurchaseRepository.ListFiltered: %w", err)
	}

	rows, err = scanPurchases(queried)
	if err != nil {
		return nil, 0, fmt.Errorf("infra.PurchaseRepository.ListFiltered: %w", err)
	}

	return rows, total, nil
}

func (r *PurchaseRepository) Earnings(ctx context.Context, dayStart, weekStart, monthStart time.Time) (domain.EarningsSummary, error) {
	const query = `SELECT
		COALESCE(SUM(amount) FILTER (WHERE completed_at >= $1), 0),
		COALESCE(SUM(amount) FILTER (WHERE completed_at >= $2), 0),
		COALESCE(SUM(amount) FILTER (WHERE completed_at >= $3), 0),
		COALESCE(SUM(amount), 0)
	FROM cp_purchases WHERE status = 2`

	var summary domain.EarningsSummary
	if err := r.Pool.QueryRow(ctx, query, dayStart, weekStart, monthStart).Scan(
		&summary.Today, &summary.Week, &summary.Month, &summary.AllTime,
	); err != nil {
		return domain.EarningsSummary{}, fmt.Errorf("infra.PurchaseRepository.Earnings: %w", err)
	}

	return summary, nil
}

func scanPurchase(row pgx.Row) (domain.Purchase, error) {
	var purchase domain.Purchase
	if err := row.Scan(
		&purchase.ID, &purchase.AccountID, &purchase.PackageKey, &purchase.Provider,
		&purchase.ProviderRef, &purchase.ProviderPaymentID, &purchase.Amount, &purchase.Currency,
		&purchase.CashPoints, &purchase.Status, &purchase.CreatedAt, &purchase.CompletedAt, &purchase.DisputedAt,
	); err != nil {
		return domain.Purchase{}, fmt.Errorf("infra.scanPurchase: %w", err)
	}

	return purchase, nil
}

func scanPurchases(rows pgx.Rows) ([]domain.Purchase, error) {
	defer rows.Close()

	out := []domain.Purchase{}
	for rows.Next() {
		purchase, err := scanPurchase(rows)
		if err != nil {
			return nil, fmt.Errorf("infra.scanPurchases scan: %w", err)
		}
		out = append(out, purchase)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("infra.scanPurchases rows: %w", err)
	}

	return out, nil
}
