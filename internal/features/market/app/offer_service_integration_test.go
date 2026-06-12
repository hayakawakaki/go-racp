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
	legs, err := settlementRepo.Pending(ctx, 10)
	if err != nil {
		t.Fatalf("Pending: %v", err)
	}
	if len(legs) != 1 {
		t.Fatalf("settlement legs = %d, want 1 (give item to buyer)", len(legs))
	}
	if legs[0].RecipientAccountID != 2 || legs[0].EscrowRef != ref || legs[0].DeliverAmount != 2 || legs[0].Whole {
		t.Errorf("give leg = %+v, want recipient 2, ref %d, amount 2 (1 unit * 2), partial (stackable)", legs[0], ref)
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

	return seedSellListing(t, repo, 1, 1000000000000)
}

func TestOfferService_Take_BuyOrder(t *testing.T) {
	pool := testutil.OpenPostgres(t, "DB_CP_TEST_URL")
	testutil.LockPostgres(t, pool, testutil.CurrencyLockKey)
	testutil.TruncatePostgres(t, pool, "cp_listing")
	testutil.TruncatePostgres(t, pool, "cp_settlement")
	testutil.TruncatePostgres(t, pool, "cp_currency")
	testutil.TruncatePostgres(t, pool, "cp_currency_hold")
	ctx := context.Background()

	listingRepo := infra.NewListingRepository(pool)
	walletRepo := infra.NewWalletRepository(pool)
	settlementRepo := infra.NewSettlementRepository(pool)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	takerStash := &fakeStashRepository{locked: true, items: []domain.StashItem{{ID: 50, NameID: 501, Amount: 100}}}
	service := NewOfferService(listingRepo, takerStash, &stubEscrow{}, walletRepo, settlementRepo, pool, logger)

	if _, err := pool.Exec(ctx, `INSERT INTO cp_currency (account_id, zeny, cashpoint) VALUES ($1, $2, 0)`, 1, 1000000); err != nil {
		t.Fatalf("seed poster: %v", err)
	}

	ref, err := service.Create(ctx, CreateInput{
		SellerAccountID: 1, Kind: domain.KindBuy, WantNameID: 501, WantUnitAmount: 1, GiveZeny: 10000, Quantity: 3,
	})
	if err != nil {
		t.Fatalf("Create buy order: %v", err)
	}

	if takeErr := service.Take(ctx, TakeInput{ListingID: ref, TakerAccountID: 2, Units: 1, TakerStashItemID: 50}); takeErr != nil {
		t.Fatalf("Take: %v", takeErr)
	}

	if taker, _ := walletRepo.Balance(ctx, 2); taker.Zeny != 9800 {
		t.Errorf("taker (seller) zeny = %d, want 9800 (10000 minus 2%% fee)", taker.Zeny)
	}

	var heldZeny int64
	if scanErr := pool.QueryRow(ctx, `SELECT zeny FROM cp_currency_hold`).Scan(&heldZeny); scanErr != nil {
		t.Fatalf("read hold: %v", scanErr)
	}
	if heldZeny != 20000 {
		t.Errorf("remaining hold = %d, want 20000 (30000 minus 10000 settled)", heldZeny)
	}

	legs, err := settlementRepo.Pending(ctx, 10)
	if err != nil {
		t.Fatalf("Pending: %v", err)
	}
	if len(legs) != 1 {
		t.Fatalf("settlement legs = %d, want 1 (taker item delivered to poster)", len(legs))
	}
	if legs[0].RecipientAccountID != 1 || !legs[0].Whole {
		t.Errorf("want leg = %+v, want recipient 1 (poster), whole", legs[0])
	}
}
