package app

import (
	"context"
	"io"
	"log/slog"
	"slices"
	"testing"

	"github.com/hayakawakaki/go-racp/internal/features/market/domain"
)

func buildWorkers(
	listings *stubListings,
	escrow *stubEscrow,
	wallet *stubWallet,
	settlement *stubSettlement,
	opts ...WorkerOption,
) *Workers {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	return NewWorkers(listings, escrow, wallet, settlement, logger, opts...)
}

func TestWorkers_Deliver(t *testing.T) {
	t.Parallel()

	settlement := &stubSettlement{pending: []domain.SettlementLeg{
		{ID: 1, EscrowRef: 10, RecipientAccountID: 5, Whole: true},
		{ID: 2, EscrowRef: 11, RecipientAccountID: 6, DeliverAmount: 5, Whole: false},
		{ID: 3, EscrowRef: 12, RecipientAccountID: 7, Whole: true},
	}}
	escrow := &stubEscrow{deliverErrs: map[int64]error{12: domain.ErrStorageUnlocked}}
	workers := buildWorkers(&stubListings{}, escrow, &stubWallet{}, settlement)

	count, err := workers.Deliver(context.Background())
	if err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if count != 2 {
		t.Errorf("delivered count = %d, want 2 (leg 3 left pending on locked storage)", count)
	}
	if !slices.Contains(escrow.delivered, 10) || !slices.Contains(escrow.delivered, 11) {
		t.Errorf("delivered refs = %v, want both 10 (whole) and 11 (partial) attempted", escrow.delivered)
	}
	if len(settlement.doneIDs) != 2 || slices.Contains(settlement.doneIDs, 3) {
		t.Errorf("doneIDs = %v, want exactly legs 1 and 2 (leg 3 not marked done)", settlement.doneIDs)
	}
}

func TestWorkers_Reconcile_ReturnsOnlyTrueOrphans(t *testing.T) {
	t.Parallel()

	escrow := &stubEscrow{orphanRefs: []int64{10, 11, 12, 13}}
	listings := &stubListings{allRefs: []int64{10}}
	settlement := &stubSettlement{pendingRefs: []int64{11}}
	workers := buildWorkers(listings, escrow, &stubWallet{}, settlement)

	count, err := workers.Reconcile(context.Background())
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if count != 2 {
		t.Errorf("returned count = %d, want 2 (refs 12 and 13)", count)
	}
	if slices.Contains(escrow.returned, 10) {
		t.Error("returned ref 10, which is a live listing id and must be skipped")
	}
	if slices.Contains(escrow.returned, 11) {
		t.Error("returned ref 11, which has a pending settlement leg and must be skipped")
	}
	if !slices.Contains(escrow.returned, 12) || !slices.Contains(escrow.returned, 13) {
		t.Errorf("returned = %v, want true orphans 12 and 13", escrow.returned)
	}
}

func TestWorkers_Reconcile_NoOrphans(t *testing.T) {
	t.Parallel()

	workers := buildWorkers(&stubListings{}, &stubEscrow{}, &stubWallet{}, &stubSettlement{})

	count, err := workers.Reconcile(context.Background())
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}

func TestWorkers_Expire(t *testing.T) {
	t.Parallel()

	holdID := int64(55)
	listings := &stubListings{dueListings: []domain.Listing{
		{ID: 7, SellerAccountID: 5, GiveItem: true, Stackable: true, GiveUnitAmount: 1, RemainingQuantity: 30},
		{ID: 8, SellerAccountID: 6, GiveHoldID: &holdID},
	}}
	settlement := &stubSettlement{}
	wallet := &stubWallet{}
	workers := buildWorkers(listings, &stubEscrow{}, wallet, settlement)

	count, err := workers.Expire(context.Background())
	if err != nil {
		t.Fatalf("Expire: %v", err)
	}
	if count != 2 {
		t.Errorf("expired count = %d, want 2", count)
	}
	if listings.statusSet != domain.StatusExpired {
		t.Errorf("status set = %d, want StatusExpired", listings.statusSet)
	}
	if len(settlement.legs) != 1 {
		t.Fatalf("legs = %d, want 1 (item return for listing 7)", len(settlement.legs))
	}
	leg := settlement.legs[0]
	if leg.EscrowRef != 7 || leg.DeliverAmount != 30 || leg.Whole {
		t.Errorf("return leg = %+v, want ref 7, amount 30 (unsold remainder), partial", leg)
	}
	if wallet.released != 55 {
		t.Errorf("released hold = %d, want 55", wallet.released)
	}
}
