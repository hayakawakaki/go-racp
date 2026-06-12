package infra

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/market/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const listingColumns = `id, seller_account_id, kind, status,
	give_item, give_nameid, give_refine, give_grade, give_card0, give_card1, give_card2, give_card3,
	give_unit_amount, give_zeny, give_cashpoint, give_hold_id,
	want_nameid, want_unit_amount,
	want_zeny, want_cashpoint,
	total_quantity, remaining_quantity, stackable, created_at, expires_at`

type ListingRepository struct {
	Pool *pgxpool.Pool
}

func NewListingRepository(pool *pgxpool.Pool) *ListingRepository {
	return &ListingRepository{Pool: pool}
}

func scanListing(row pgx.Row) (domain.Listing, error) {
	var l domain.Listing
	err := row.Scan(
		&l.ID, &l.SellerAccountID, &l.Kind, &l.Status,
		&l.GiveItem, &l.GiveNameID, &l.GiveRefine, &l.GiveGrade, &l.GiveCard[0], &l.GiveCard[1], &l.GiveCard[2], &l.GiveCard[3],
		&l.GiveUnitAmount, &l.GiveZeny, &l.GiveCashpoint, &l.GiveHoldID,
		&l.WantNameID, &l.WantUnitAmount,
		&l.WantZeny, &l.WantCashpoint,
		&l.TotalQuantity, &l.RemainingQuantity, &l.Stackable, &l.CreatedAt, &l.ExpiresAt,
	)
	if err != nil {
		return domain.Listing{}, fmt.Errorf("infra.scanListing: %w", err)
	}

	return l, nil
}

func (r *ListingRepository) NextRef(ctx context.Context) (int64, error) {
	var ref int64
	if err := r.Pool.QueryRow(ctx, `SELECT nextval('cp_market_ref_seq')`).Scan(&ref); err != nil {
		return 0, fmt.Errorf("infra.ListingRepository.NextRef: %w", err)
	}

	return ref, nil
}

func (r *ListingRepository) Create(ctx context.Context, l domain.Listing) error {
	_, err := r.Pool.Exec(ctx,
		`INSERT INTO cp_listing (
			id, seller_account_id, kind, status,
			give_item, give_nameid, give_refine, give_grade, give_card0, give_card1, give_card2, give_card3,
			give_unit_amount, give_zeny, give_cashpoint, give_hold_id,
			want_nameid, want_unit_amount,
			want_zeny, want_cashpoint,
			total_quantity, remaining_quantity, stackable, expires_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24)`,
		l.ID, l.SellerAccountID, l.Kind, domain.StatusActive,
		l.GiveItem, l.GiveNameID, l.GiveRefine, l.GiveGrade, l.GiveCard[0], l.GiveCard[1], l.GiveCard[2], l.GiveCard[3],
		l.GiveUnitAmount, l.GiveZeny, l.GiveCashpoint, l.GiveHoldID,
		l.WantNameID, l.WantUnitAmount,
		l.WantZeny, l.WantCashpoint,
		l.TotalQuantity, l.RemainingQuantity, l.Stackable, l.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("infra.ListingRepository.Create: %w", err)
	}

	return nil
}

func (r *ListingRepository) Get(ctx context.Context, id int64) (domain.Listing, error) {
	listing, err := scanListing(r.Pool.QueryRow(ctx, `SELECT `+listingColumns+` FROM cp_listing WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Listing{}, domain.ErrListingNotFound
	}
	if err != nil {
		return domain.Listing{}, fmt.Errorf("infra.ListingRepository.Get: %w", err)
	}

	return listing, nil
}

func (r *ListingRepository) Browse(ctx context.Context, kind, limit, offset int) ([]domain.Listing, int, error) {
	return r.listPage(ctx, `status = 1 AND ($1 = 0 OR kind = $1)`, kind, limit, offset)
}

func (r *ListingRepository) BySeller(ctx context.Context, accountID, limit, offset int) ([]domain.Listing, int, error) {
	return r.listPage(ctx, `seller_account_id = $1`, accountID, limit, offset)
}

func (r *ListingRepository) listPage(ctx context.Context, where string, whereArg any, limit, offset int) ([]domain.Listing, int, error) {
	var total int
	if err := r.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM cp_listing WHERE `+where, whereArg).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("infra.ListingRepository.listPage count: %w", err)
	}

	rows, err := r.Pool.Query(ctx,
		`SELECT `+listingColumns+` FROM cp_listing WHERE `+where+` ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		whereArg, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("infra.ListingRepository.listPage: %w", err)
	}
	defer rows.Close()

	out, err := collectListings(rows)
	if err != nil {
		return nil, 0, err
	}

	return out, total, nil
}

func (r *ListingRepository) TakeUnits(ctx context.Context, id int64, units int) (domain.Listing, bool, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return domain.Listing{}, false, fmt.Errorf("infra.ListingRepository.TakeUnits begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	listing, err := scanListing(tx.QueryRow(ctx, `SELECT `+listingColumns+` FROM cp_listing WHERE id = $1 FOR UPDATE`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Listing{}, false, domain.ErrListingNotFound
	}
	if err != nil {
		return domain.Listing{}, false, fmt.Errorf("infra.ListingRepository.TakeUnits read: %w", err)
	}
	if listing.Status != domain.StatusActive {
		return domain.Listing{}, false, domain.ErrListingInactive
	}
	if units <= 0 || units > listing.RemainingQuantity {
		return domain.Listing{}, false, domain.ErrInsufficientUnits
	}

	remaining := listing.RemainingQuantity - units
	status := domain.StatusActive
	depleted := remaining == 0
	if depleted {
		status = domain.StatusTaken
	}

	if _, err = tx.Exec(ctx,
		`UPDATE cp_listing SET remaining_quantity = $1, status = $2 WHERE id = $3`,
		remaining, status, id,
	); err != nil {
		return domain.Listing{}, false, fmt.Errorf("infra.ListingRepository.TakeUnits update: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Listing{}, false, fmt.Errorf("infra.ListingRepository.TakeUnits commit: %w", err)
	}

	listing.RemainingQuantity = remaining
	listing.Status = status

	return listing, depleted, nil
}

func (r *ListingRepository) SetStatus(ctx context.Context, id int64, status int) error {
	if _, err := r.Pool.Exec(ctx, `UPDATE cp_listing SET status = $1 WHERE id = $2`, status, id); err != nil {
		return fmt.Errorf("infra.ListingRepository.SetStatus: %w", err)
	}

	return nil
}

func (r *ListingRepository) DueForExpiry(ctx context.Context, now time.Time, limit int) ([]domain.Listing, error) {
	rows, err := r.Pool.Query(ctx,
		`SELECT `+listingColumns+` FROM cp_listing WHERE status = 1 AND expires_at < $1 ORDER BY expires_at LIMIT $2`,
		now, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("infra.ListingRepository.DueForExpiry: %w", err)
	}
	defer rows.Close()

	return collectListings(rows)
}

func (r *ListingRepository) AllRefs(ctx context.Context) ([]int64, error) {
	rows, err := r.Pool.Query(ctx, `SELECT id FROM cp_listing`)
	if err != nil {
		return nil, fmt.Errorf("infra.ListingRepository.AllRefs: %w", err)
	}
	defer rows.Close()

	out := make([]int64, 0)
	for rows.Next() {
		var ref int64
		if err := rows.Scan(&ref); err != nil {
			return nil, fmt.Errorf("infra.ListingRepository.AllRefs scan: %w", err)
		}
		out = append(out, ref)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("infra.ListingRepository.AllRefs rows: %w", err)
	}

	return out, nil
}

func collectListings(rows pgx.Rows) ([]domain.Listing, error) {
	out := make([]domain.Listing, 0)
	for rows.Next() {
		listing, err := scanListing(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, listing)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("infra.collectListings: %w", err)
	}

	return out, nil
}
