//go:build integration

package app

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/market/domain"
	"github.com/hayakawakaki/go-racp/internal/features/market/infra"
	"github.com/hayakawakaki/go-racp/internal/testutil"
)

func TestOfferService_Take_AtomicSettlement(t *testing.T) {
	pool := testutil.OpenPostgres(t, "DB_CP_TEST_URL")
	testutil.LockPostgres(t, pool, testutil.CurrencyLockKey)
	testutil.TruncatePostgres(t, pool, "cp_listing")
	testutil.TruncatePostgres(t, pool, "cp_settlement")
	testutil.TruncatePostgres(t, pool, "cp_currency")
	ctx := context.Background()

	listingRepo := infra.NewListingRepository(pool)
	walletRepo := infra.NewWalletRepository(pool)
	settlementRepo := infra.NewSettlementRepository(pool)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	service := NewOfferService(
		listingRepo,
		&fakeStashRepository{locked: true},
		&stubEscrow{},
		walletRepo,
		settlementRepo,
		pool,
		logger,
	)

	if _, err := pool.Exec(ctx, `INSERT INTO cp_currency (account_id, zeny, cashpoint) VALUES ($1, $2, 0)`, 2, 1000000); err != nil {
		t.Fatalf("seed buyer: %v", err)
	}

	ref := seedSellListing(t, listingRepo, 5, 10000)

	if err := service.Take(ctx, TakeInput{ListingID: ref, TakerAccountID: 2, Units: 2}); err != nil {
		t.Fatalf("Take: %v", err)
	}

	got, err := listingRepo.Get(ctx, ref)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.RemainingQuantity != 3 {
		t.Errorf("remaining = %d, want 3", got.RemainingQuantity)
	}
	if buyer, _ := walletRepo.Balance(ctx, 2); buyer.Zeny != 980000 {
		t.Errorf("buyer zeny = %d, want 980000 (charged 20000)", buyer.Zeny)
	}
	if seller, _ := walletRepo.Balance(ctx, 1); seller.Zeny != 19600 {
		t.Errorf("seller zeny = %d, want 19600 (20000 minus 2%% fee)", seller.Zeny)
	}
	if legs, _ := settlementRepo.Pending(ctx, 10); len(legs) != 1 {
		t.Errorf("settlement legs = %d, want 1 (give item to buyer)", len(legs))
	}

	bigRef := seedUnaffordableListing(t, listingRepo)
	buyerBefore, _ := walletRepo.Balance(ctx, 2)

	if err := service.Take(ctx, TakeInput{ListingID: bigRef, TakerAccountID: 2, Units: 1}); err == nil {
		t.Fatal("Take of an unaffordable listing: expected error")
	}

	afterListing, _ := listingRepo.Get(ctx, bigRef)
	if afterListing.RemainingQuantity != 1 || afterListing.Status != domain.StatusActive {
		t.Errorf("after failed take: remaining %d status %d, want 1 active (rolled back)", afterListing.RemainingQuantity, afterListing.Status)
	}
	if buyerAfter, _ := walletRepo.Balance(ctx, 2); buyerAfter.Zeny != buyerBefore.Zeny {
		t.Errorf("buyer zeny changed on failed take, want unchanged (rolled back)")
	}
	if legsAfter, _ := settlementRepo.Pending(ctx, 10); len(legsAfter) != 1 {
		t.Errorf("settlement legs after failed take = %d, want still 1 (no new leg)", len(legsAfter))
	}
}

func seedSellListing(t *testing.T, repo *infra.ListingRepository, quantity int, wantZeny int64) int64 {
	t.Helper()
	ctx := context.Background()
	ref, err := repo.NextRef(ctx)
	if err != nil {
		t.Fatalf("NextRef: %v", err)
	}
	listing := domain.Listing{
		ID:                ref,
		SellerAccountID:   1,
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
		t.Fatalf("Create listing: %v", err)
	}

	return ref
}

func seedUnaffordableListing(t *testing.T, repo *infra.ListingRepository) int64 {
	t.Helper()
	ctx := context.Background()
	ref, err := repo.NextRef(ctx)
	if err != nil {
		t.Fatalf("NextRef: %v", err)
	}
	listing := domain.Listing{
		ID:                ref,
		SellerAccountID:   1,
		Kind:              domain.KindSell,
		Status:            domain.StatusActive,
		GiveItem:          true,
		GiveNameID:        501,
		GiveUnitAmount:    1,
		WantZeny:          1000000000000,
		TotalQuantity:     1,
		RemainingQuantity: 1,
		Stackable:         true,
		ExpiresAt:         time.Now().Add(time.Hour),
	}
	if err := repo.Create(ctx, listing); err != nil {
		t.Fatalf("Create unaffordable listing: %v", err)
	}

	return ref
}
