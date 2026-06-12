//go:build integration

package infra

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/market/domain"
	"github.com/hayakawakaki/go-racp/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ domain.ListingRepository = (*ListingRepository)(nil)

func setupListingRepo(t *testing.T) (*ListingRepository, *pgxpool.Pool) {
	t.Helper()
	pool := testutil.OpenPostgres(t, "DB_CP_TEST_URL")
	testutil.LockPostgres(t, pool, testutil.CurrencyLockKey)
	testutil.TruncatePostgres(t, pool, "cp_listing")

	return NewListingRepository(pool), pool
}

func createSellListing(t *testing.T, repo *ListingRepository, seller, quantity int, wantZeny int64) int64 {
	t.Helper()
	ctx := context.Background()
	ref, err := repo.NextRef(ctx)
	if err != nil {
		t.Fatalf("NextRef: %v", err)
	}
	listing := domain.Listing{
		ID:                ref,
		SellerAccountID:   seller,
		Kind:              domain.KindSell,
		Status:            domain.StatusActive,
		GiveItem:          true,
		GiveNameID:        501,
		GiveUnitAmount:    1,
		WantZeny:          wantZeny,
		TotalQuantity:     quantity,
		RemainingQuantity: quantity,
		Stackable:         true,
		ExpiresAt:         time.Now().Add(time.Hour),
	}
	if err := repo.Create(ctx, listing); err != nil {
		t.Fatalf("Create: %v", err)
	}

	return ref
}

func TestListingRepository_CreateGet(t *testing.T) {
	repo, _ := setupListingRepo(t)
	ctx := context.Background()
	ref := createSellListing(t, repo, 1, 5, 1000)

	got, err := repo.Get(ctx, ref)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.SellerAccountID != 1 || got.Kind != domain.KindSell || got.WantZeny != 1000 || got.RemainingQuantity != 5 || !got.GiveItem || !got.Stackable {
		t.Errorf("Get = %+v, want seller 1 sell wantZeny 1000 remaining 5 give-item stackable", got)
	}

	if _, missErr := repo.Get(ctx, 999999); !errors.Is(missErr, domain.ErrListingNotFound) {
		t.Errorf("Get missing err = %v, want ErrListingNotFound", missErr)
	}
}

func TestListingRepository_TakeUnitsNoOversell(t *testing.T) {
	repo, _ := setupListingRepo(t)
	ctx := context.Background()
	ref := createSellListing(t, repo, 1, 5, 1000)

	listing, depleted, err := repo.TakeUnits(ctx, ref, 3)
	if err != nil {
		t.Fatalf("TakeUnits 3: %v", err)
	}
	if depleted || listing.RemainingQuantity != 2 {
		t.Errorf("after take 3: remaining %d depleted %v, want remaining 2 not depleted", listing.RemainingQuantity, depleted)
	}

	if _, _, oversellErr := repo.TakeUnits(ctx, ref, 3); !errors.Is(oversellErr, domain.ErrInsufficientUnits) {
		t.Errorf("take 3 of 2 remaining err = %v, want ErrInsufficientUnits", oversellErr)
	}

	listing, depleted, err = repo.TakeUnits(ctx, ref, 2)
	if err != nil {
		t.Fatalf("TakeUnits 2: %v", err)
	}
	if !depleted || listing.RemainingQuantity != 0 || listing.Status != domain.StatusTaken {
		t.Errorf("after final take: remaining %d depleted %v status %d, want 0 depleted taken", listing.RemainingQuantity, depleted, listing.Status)
	}

	if _, _, inactiveErr := repo.TakeUnits(ctx, ref, 1); !errors.Is(inactiveErr, domain.ErrListingInactive) {
		t.Errorf("take from depleted listing err = %v, want ErrListingInactive", inactiveErr)
	}
}

func TestListingRepository_BrowseSellerAndExpiry(t *testing.T) {
	repo, pool := setupListingRepo(t)
	ctx := context.Background()
	sellRef := createSellListing(t, repo, 1, 5, 1000)
	createSellListing(t, repo, 2, 1, 500)

	rows, total, err := repo.Browse(ctx, domain.KindSell, 10, 0)
	if err != nil {
		t.Fatalf("Browse: %v", err)
	}
	if total != 2 || len(rows) != 2 {
		t.Errorf("Browse total %d rows %d, want 2/2", total, len(rows))
	}

	mine, mineTotal, err := repo.BySeller(ctx, 1, 10, 0)
	if err != nil {
		t.Fatalf("BySeller: %v", err)
	}
	if mineTotal != 1 || len(mine) != 1 || mine[0].SellerAccountID != 1 {
		t.Errorf("BySeller(1) = total %d rows %d, want 1/1 for seller 1", mineTotal, len(mine))
	}

	if _, execErr := pool.Exec(ctx, `UPDATE cp_listing SET expires_at = NOW() - INTERVAL '1 hour' WHERE id = $1`, sellRef); execErr != nil {
		t.Fatalf("force expiry: %v", execErr)
	}
	due, err := repo.DueForExpiry(ctx, time.Now(), 10)
	if err != nil {
		t.Fatalf("DueForExpiry: %v", err)
	}
	if len(due) != 1 || due[0].ID != sellRef {
		t.Errorf("DueForExpiry = %+v, want only listing %d", due, sellRef)
	}

	refs, err := repo.AllRefs(ctx)
	if err != nil {
		t.Fatalf("AllRefs: %v", err)
	}
	if len(refs) != 2 {
		t.Errorf("AllRefs = %d, want 2", len(refs))
	}
}
