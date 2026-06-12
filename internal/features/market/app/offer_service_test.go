package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"math"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/market/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var errOfferTest = errors.New("app: offer test error")

var (
	_ domain.ListingRepository    = (*stubListings)(nil)
	_ domain.EscrowRepository     = (*stubEscrow)(nil)
	_ domain.WalletRepository     = (*stubWallet)(nil)
	_ domain.SettlementRepository = (*stubSettlement)(nil)
	_ txBeginner                  = stubTx{}
)

type stubListings struct {
	getErr       error
	createErr    error
	setStatusErr error
	created      *domain.Listing
	dueListings  []domain.Listing
	allRefs      []int64
	getListing   domain.Listing
	nextRef      int64
	statusSet    int
}

func (s *stubListings) NextRef(context.Context) (int64, error) { return s.nextRef, nil }

func (s *stubListings) Create(_ context.Context, listing domain.Listing) error {
	stored := listing
	s.created = &stored

	return s.createErr
}

func (s *stubListings) Get(context.Context, int64) (domain.Listing, error) {
	return s.getListing, s.getErr
}

func (s *stubListings) Browse(context.Context, int, int, int) ([]domain.Listing, int, error) {
	return nil, 0, nil
}

func (s *stubListings) BySeller(context.Context, int, int, int) ([]domain.Listing, int, error) {
	return nil, 0, nil
}

func (s *stubListings) TakeUnits(context.Context, int64, int) (domain.Listing, bool, error) {
	return domain.Listing{}, false, nil
}

func (s *stubListings) TakeUnitsTx(context.Context, domain.DBTX, int64, int) (domain.Listing, bool, error) {
	return domain.Listing{}, false, nil
}

func (s *stubListings) SetStatus(_ context.Context, _ int64, status int) error {
	s.statusSet = status

	return s.setStatusErr
}

func (s *stubListings) SetStatusTx(_ context.Context, _ domain.DBTX, _ int64, status int) error {
	s.statusSet = status

	return s.setStatusErr
}

func (s *stubListings) DueForExpiry(context.Context, time.Time, int) ([]domain.Listing, error) {
	return s.dueListings, nil
}

func (s *stubListings) AllRefs(context.Context) ([]int64, error) { return s.allRefs, nil }

type escrowCall struct {
	moves      []domain.EscrowMove
	listingRef int64
	accountID  int
}

type partialDelivery struct {
	ref    int64
	amount int
}

type stubEscrow struct {
	moveErr          error
	returnErr        error
	deliverErrs      map[int64]error
	calls            []escrowCall
	wholeDelivered   []int64
	partialDelivered []partialDelivery
	returned         []int64
	orphanRefs       []int64
}

func (s *stubEscrow) MoveToEscrow(_ context.Context, accountID int, listingRef int64, moves []domain.EscrowMove) error {
	s.calls = append(s.calls, escrowCall{accountID: accountID, listingRef: listingRef, moves: moves})

	return s.moveErr
}

func (s *stubEscrow) ReturnToStash(_ context.Context, ref int64) error {
	s.returned = append(s.returned, ref)

	return s.returnErr
}

func (s *stubEscrow) Deliver(_ context.Context, ref int64, _ int, _ int64) error {
	s.wholeDelivered = append(s.wholeDelivered, ref)

	return s.deliverErrs[ref]
}

func (s *stubEscrow) DeliverPartial(_ context.Context, ref int64, _, amount int, _ int64) error {
	s.partialDelivered = append(s.partialDelivered, partialDelivery{ref: ref, amount: amount})

	return s.deliverErrs[ref]
}

func (s *stubEscrow) ByListing(context.Context, int64) ([]domain.StashItem, error) {
	return nil, nil
}

func (s *stubEscrow) OrphanRefs(context.Context, time.Time) ([]int64, error) {
	return s.orphanRefs, nil
}

type stubWallet struct {
	holdErr       error
	burnErr       error
	releaseErr    error
	holdID        int64
	holdZeny      int64
	burnZeny      int64
	released      int64
	holdCashpoint int
	burnCalled    bool
}

func (s *stubWallet) Balance(context.Context, int) (domain.Wallet, error) {
	return domain.Wallet{}, nil
}

func (s *stubWallet) Hold(_ context.Context, _ int, zeny int64, cashpoint int) (int64, error) {
	s.holdZeny = zeny
	s.holdCashpoint = cashpoint

	return s.holdID, s.holdErr
}

func (s *stubWallet) Release(_ context.Context, holdID int64) error {
	s.released = holdID

	return s.releaseErr
}

func (s *stubWallet) ReleaseTx(_ context.Context, _ domain.DBTX, holdID int64) error {
	s.released = holdID

	return s.releaseErr
}

func (s *stubWallet) SettleHold(context.Context, int64, int, int64, int) error { return nil }

func (s *stubWallet) SettleHoldPartial(context.Context, int64, int, int64, int, int64, int) error {
	return nil
}

func (s *stubWallet) ChargeTx(context.Context, domain.DBTX, int, int, int64, int, int64, int) error {
	return nil
}

func (s *stubWallet) SettleHoldPartialTx(context.Context, domain.DBTX, int64, int, int64, int, int64, int) error {
	return nil
}

func (s *stubWallet) Charge(context.Context, int, int, int64, int, int64, int) error { return nil }

func (s *stubWallet) Burn(_ context.Context, _ int, zeny int64, _ int) error {
	s.burnCalled = true
	s.burnZeny = zeny

	return s.burnErr
}

type stubSettlement struct {
	enqueueErr  error
	legs        []domain.SettlementLeg
	pending     []domain.SettlementLeg
	pendingRefs []int64
	doneIDs     []int64
}

func (s *stubSettlement) Enqueue(_ context.Context, leg domain.SettlementLeg) error {
	s.legs = append(s.legs, leg)

	return s.enqueueErr
}

func (s *stubSettlement) EnqueueTx(_ context.Context, _ domain.DBTX, leg domain.SettlementLeg) error {
	s.legs = append(s.legs, leg)

	return s.enqueueErr
}

func (s *stubSettlement) Pending(context.Context, int) ([]domain.SettlementLeg, error) {
	return s.pending, nil
}

func (s *stubSettlement) PendingRefs(context.Context) ([]int64, error) { return s.pendingRefs, nil }

func (s *stubSettlement) MarkDone(_ context.Context, id int64) error {
	s.doneIDs = append(s.doneIDs, id)

	return nil
}

type stubTx struct{}

func (stubTx) Begin(context.Context) (pgx.Tx, error) { return stubTx{}, nil }

func (stubTx) Commit(context.Context) error   { return nil }
func (stubTx) Rollback(context.Context) error { return nil }

func (stubTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, nil
}

func (stubTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { return nil }

func (stubTx) LargeObjects() pgx.LargeObjects { return pgx.LargeObjects{} }

func (stubTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, nil
}

func (stubTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (stubTx) Query(context.Context, string, ...any) (pgx.Rows, error) { return nil, nil }

func (stubTx) QueryRow(context.Context, string, ...any) pgx.Row { return nil }

func (stubTx) Conn() *pgx.Conn { return nil }

type stubBlacklist struct{ blocked map[int]bool }

func (b stubBlacklist) Blocked(nameID int) bool { return b.blocked[nameID] }

func buildOffer(
	listings *stubListings,
	stash *fakeStashRepository,
	escrow *stubEscrow,
	wallet *stubWallet,
	settlement *stubSettlement,
	opts ...Option,
) *OfferService {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	return NewOfferService(listings, stash, escrow, wallet, settlement, stubTx{}, logger, opts...)
}

func TestOfferService_Create_Validation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   CreateInput
	}{
		{name: "zero quantity", in: CreateInput{Kind: domain.KindSell, Quantity: 0, GiveUnitAmount: 1, WantZeny: 100}},
		{name: "unknown kind", in: CreateInput{Kind: 99, Quantity: 1, GiveUnitAmount: 1, WantZeny: 100}},
		{name: "sell zero price", in: CreateInput{Kind: domain.KindSell, Quantity: 1, GiveUnitAmount: 1}},
		{name: "sell zero give amount", in: CreateInput{Kind: domain.KindSell, Quantity: 1, WantZeny: 100}},
		{name: "buy no want nameid", in: CreateInput{Kind: domain.KindBuy, Quantity: 1, WantUnitAmount: 1, GiveZeny: 100}},
		{name: "buy zero give", in: CreateInput{Kind: domain.KindBuy, Quantity: 1, WantNameID: 501, WantUnitAmount: 1}},
		{name: "cashpoint over int32", in: CreateInput{Kind: domain.KindSell, Quantity: 1, GiveUnitAmount: 1, WantZeny: 100, GiveCashpoint: math.MaxInt32 + 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			service := buildOffer(&stubListings{}, &fakeStashRepository{locked: true}, &stubEscrow{}, &stubWallet{}, &stubSettlement{})

			_, err := service.Create(context.Background(), tt.in)
			if !errors.Is(err, domain.ErrInvalidOffer) {
				t.Errorf("Create() err = %v, want ErrInvalidOffer", err)
			}
		})
	}
}

func TestOfferService_Create_SellRequiresLock(t *testing.T) {
	t.Parallel()

	stash := &fakeStashRepository{locked: false, items: []domain.StashItem{{ID: 10, NameID: 501, Amount: 100}}}
	service := buildOffer(&stubListings{}, stash, &stubEscrow{}, &stubWallet{}, &stubSettlement{})

	_, err := service.Create(context.Background(), CreateInput{
		Kind: domain.KindSell, GiveStashItemID: 10, GiveUnitAmount: 1, Quantity: 1, WantZeny: 1000,
	})
	if !errors.Is(err, domain.ErrStorageUnlocked) {
		t.Errorf("Create() err = %v, want ErrStorageUnlocked", err)
	}
}

func TestOfferService_Create_BlacklistedItem(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   CreateInput
	}{
		{name: "sell give blacklisted", in: CreateInput{Kind: domain.KindSell, GiveStashItemID: 10, GiveUnitAmount: 1, Quantity: 1, WantZeny: 1000}},
		{name: "buy want blacklisted", in: CreateInput{Kind: domain.KindBuy, WantNameID: 501, WantUnitAmount: 1, GiveZeny: 1000, Quantity: 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			blacklist := stubBlacklist{blocked: map[int]bool{501: true}}
			stash := &fakeStashRepository{locked: true, items: []domain.StashItem{{ID: 10, NameID: 501, Amount: 100}}}
			service := buildOffer(&stubListings{}, stash, &stubEscrow{}, &stubWallet{}, &stubSettlement{}, WithBlacklist(blacklist))

			_, err := service.Create(context.Background(), tt.in)
			if !errors.Is(err, domain.ErrItemBlacklisted) {
				t.Errorf("Create() err = %v, want ErrItemBlacklisted", err)
			}
		})
	}
}

func TestOfferService_Create_SellEscrowsAndBurnsFee(t *testing.T) {
	t.Parallel()

	listings := &stubListings{nextRef: 77}
	stash := &fakeStashRepository{locked: true, items: []domain.StashItem{{ID: 10, NameID: 501, Amount: 100}}}
	escrow := &stubEscrow{}
	wallet := &stubWallet{}
	service := buildOffer(listings, stash, escrow, wallet, &stubSettlement{})

	ref, err := service.Create(context.Background(), CreateInput{
		SellerAccountID: 1, Kind: domain.KindSell, GiveStashItemID: 10, GiveUnitAmount: 2, Quantity: 5, WantZeny: 1000,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if ref != 77 {
		t.Errorf("ref = %d, want 77", ref)
	}
	if len(escrow.calls) != 1 || escrow.calls[0].accountID != 1 || escrow.calls[0].listingRef != 77 || escrow.calls[0].moves[0].Amount != 10 {
		t.Errorf("escrow call = %+v, want account 1 listingRef 77 amount 10 (give_unit_amount * quantity)", escrow.calls)
	}
	if listings.created == nil || !listings.created.GiveItem || !listings.created.Stackable {
		t.Errorf("created listing = %+v, want GiveItem and Stackable true", listings.created)
	}
	if !wallet.burnCalled || wallet.burnZeny != domain.DefaultFlatFeeZeny {
		t.Errorf("burn = (called %v, zeny %d), want flat fee %d burned", wallet.burnCalled, wallet.burnZeny, domain.DefaultFlatFeeZeny)
	}
}

func TestOfferService_Create_FailedPersistDoesNotBurn(t *testing.T) {
	t.Parallel()

	stash := &fakeStashRepository{locked: true, items: []domain.StashItem{{ID: 10, NameID: 501, Amount: 100}}}
	wallet := &stubWallet{}
	service := buildOffer(&stubListings{createErr: errOfferTest}, stash, &stubEscrow{}, wallet, &stubSettlement{})

	_, err := service.Create(context.Background(), CreateInput{
		Kind: domain.KindSell, GiveStashItemID: 10, GiveUnitAmount: 1, Quantity: 1, WantZeny: 1000,
	})
	if err == nil {
		t.Fatal("Create: expected error on persist failure")
	}
	if wallet.burnCalled {
		t.Error("flat fee was burned despite a failed listing insert, want burn only after a successful persist")
	}
}

func TestOfferService_Create_BuyHoldsCurrency(t *testing.T) {
	t.Parallel()

	listings := &stubListings{nextRef: 5}
	wallet := &stubWallet{holdID: 99}
	service := buildOffer(listings, &fakeStashRepository{}, &stubEscrow{}, wallet, &stubSettlement{})

	_, err := service.Create(context.Background(), CreateInput{
		Kind: domain.KindBuy, WantNameID: 501, WantUnitAmount: 1, GiveZeny: 500, GiveCashpoint: 50, Quantity: 4,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if wallet.holdZeny != 2000 {
		t.Errorf("held zeny = %d, want 2000 (give_zeny * quantity)", wallet.holdZeny)
	}
	if wallet.holdCashpoint != 200 {
		t.Errorf("held cashpoint = %d, want 200 (give_cashpoint * quantity)", wallet.holdCashpoint)
	}
	if listings.created == nil || listings.created.GiveHoldID == nil || *listings.created.GiveHoldID != 99 {
		t.Errorf("created.GiveHoldID = %v, want 99", listings.created)
	}
}

func TestOfferService_Create_NonStackableQuantity(t *testing.T) {
	t.Parallel()

	stash := &fakeStashRepository{locked: true, items: []domain.StashItem{{ID: 10, NameID: 1201, Amount: 1, Refine: 7}}}
	service := buildOffer(&stubListings{}, stash, &stubEscrow{}, &stubWallet{}, &stubSettlement{})

	_, err := service.Create(context.Background(), CreateInput{
		Kind: domain.KindSell, GiveStashItemID: 10, GiveUnitAmount: 1, Quantity: 2, WantZeny: 1000,
	})
	if !errors.Is(err, domain.ErrInvalidOffer) {
		t.Errorf("Create() err = %v, want ErrInvalidOffer for non stackable quantity > 1", err)
	}
}

func TestOfferService_Cancel_NotOwner(t *testing.T) {
	t.Parallel()

	listings := &stubListings{getListing: domain.Listing{ID: 7, SellerAccountID: 5, Status: domain.StatusActive}}
	service := buildOffer(listings, &fakeStashRepository{}, &stubEscrow{}, &stubWallet{}, &stubSettlement{})

	err := service.Cancel(context.Background(), 7, 9)
	if !errors.Is(err, domain.ErrListingNotFound) {
		t.Errorf("Cancel() err = %v, want ErrListingNotFound", err)
	}
}

func TestOfferService_Cancel_ReturnsUnsoldRemainder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		giveAmount int
		remaining  int
		wantAmount int
		stackable  bool
		wantWhole  bool
	}{
		{name: "stackable returns remainder as partial", giveAmount: 1, remaining: 40, wantAmount: 40, stackable: true, wantWhole: false},
		{name: "non stackable returns whole", giveAmount: 1, remaining: 1, wantAmount: 1, stackable: false, wantWhole: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			listing := domain.Listing{
				ID:                7,
				SellerAccountID:   5,
				Status:            domain.StatusActive,
				GiveItem:          true,
				Stackable:         tt.stackable,
				GiveUnitAmount:    tt.giveAmount,
				RemainingQuantity: tt.remaining,
			}
			settlement := &stubSettlement{}
			listings := &stubListings{getListing: listing}
			service := buildOffer(listings, &fakeStashRepository{}, &stubEscrow{}, &stubWallet{}, settlement)

			if err := service.Cancel(context.Background(), 7, 5); err != nil {
				t.Fatalf("Cancel: %v", err)
			}
			if listings.statusSet != domain.StatusCancelled {
				t.Errorf("status set = %d, want StatusCancelled", listings.statusSet)
			}
			if len(settlement.legs) != 1 {
				t.Fatalf("legs = %d, want 1", len(settlement.legs))
			}
			leg := settlement.legs[0]
			if leg.EscrowRef != 7 || leg.RecipientAccountID != 5 {
				t.Errorf("leg ref/recipient = %d/%d, want 7/5", leg.EscrowRef, leg.RecipientAccountID)
			}
			if leg.DeliverAmount != tt.wantAmount || leg.Whole != tt.wantWhole {
				t.Errorf("leg = (amount %d, whole %v), want (amount %d, whole %v)", leg.DeliverAmount, leg.Whole, tt.wantAmount, tt.wantWhole)
			}
		})
	}
}

func TestOfferService_Cancel_BuyOrderReleasesHold(t *testing.T) {
	t.Parallel()

	holdID := int64(55)
	listings := &stubListings{getListing: domain.Listing{ID: 9, SellerAccountID: 5, Status: domain.StatusActive, GiveHoldID: &holdID}}
	wallet := &stubWallet{}
	settlement := &stubSettlement{}
	service := buildOffer(listings, &fakeStashRepository{}, &stubEscrow{}, wallet, settlement)

	if err := service.Cancel(context.Background(), 9, 5); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if wallet.released != 55 {
		t.Errorf("released hold = %d, want 55", wallet.released)
	}
	if len(settlement.legs) != 0 {
		t.Errorf("legs = %d, want 0 (no item leg for a currency hold)", len(settlement.legs))
	}
}

func TestOfferService_Take_Guards(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr      error
		name         string
		takerItemID  int64
		status       int
		remaining    int
		giveRefine   int
		wantNameID   int
		takerAccount int
		units        int
		itemNameID   int
		slotsCap     int
		giveItem     bool
		locked       bool
	}{
		{name: "inactive listing", status: domain.StatusCancelled, remaining: 10, takerAccount: 9, units: 1, wantErr: domain.ErrListingInactive},
		{name: "self trade", status: domain.StatusActive, remaining: 10, takerAccount: 5, units: 1, wantErr: domain.ErrSelfTrade},
		{name: "zero units", status: domain.StatusActive, remaining: 10, takerAccount: 9, units: 0, wantErr: domain.ErrInsufficientUnits},
		{name: "units over remaining", status: domain.StatusActive, remaining: 10, takerAccount: 9, units: 11, wantErr: domain.ErrInsufficientUnits},
		{name: "sell taker storage full", status: domain.StatusActive, remaining: 10, takerAccount: 9, units: 1, giveItem: true, giveRefine: 7, slotsCap: 1, wantErr: domain.ErrStorageFull},
		{name: "buy take taker unlocked", status: domain.StatusActive, remaining: 10, takerAccount: 9, units: 1, wantNameID: 501, takerItemID: 20, locked: false, wantErr: domain.ErrStorageUnlocked},
		{name: "buy take want mismatch", status: domain.StatusActive, remaining: 10, takerAccount: 9, units: 1, wantNameID: 501, takerItemID: 20, itemNameID: 999, locked: true, wantErr: domain.ErrWantMismatch},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			listing := domain.Listing{
				SellerAccountID:   5,
				Status:            tt.status,
				RemainingQuantity: tt.remaining,
				GiveItem:          tt.giveItem,
				GiveUnitAmount:    1,
				GiveRefine:        tt.giveRefine,
				WantNameID:        tt.wantNameID,
				WantUnitAmount:    1,
			}
			stash := &fakeStashRepository{locked: tt.locked}
			var opts []Option
			if tt.slotsCap > 0 {
				stash.items = []domain.StashItem{{ID: 1}}
				opts = append(opts, WithCapacity(tt.slotsCap, domain.DefaultMaxStackAmount))
			}
			if tt.itemNameID != 0 {
				stash.items = []domain.StashItem{{ID: tt.takerItemID, NameID: tt.itemNameID, Amount: 100}}
			}
			listings := &stubListings{getListing: listing}
			service := buildOffer(listings, stash, &stubEscrow{}, &stubWallet{}, &stubSettlement{}, opts...)

			err := service.Take(context.Background(), TakeInput{
				ListingID: 1, TakerAccountID: tt.takerAccount, Units: tt.units, TakerStashItemID: tt.takerItemID,
			})
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Take() err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestOfferService_ensureCapacity(t *testing.T) {
	t.Parallel()

	stackable := domain.StashItem{NameID: 501, Amount: 5}

	tests := []struct {
		wantErr  error
		stash    *fakeStashRepository
		name     string
		snapshot domain.StashItem
		deliver  int
		slotsCap int
		stackCap int
	}{
		{name: "merge fast-path skips slot check", stash: &fakeStashRepository{items: []domain.StashItem{{ID: 1}}, mergeFound: true, mergeExisting: 10}, snapshot: stackable, deliver: 5, slotsCap: 1, stackCap: 100, wantErr: nil},
		{name: "merge over cap falls through to full slot check", stash: &fakeStashRepository{items: []domain.StashItem{{ID: 1}}, mergeFound: true, mergeExisting: 99}, snapshot: stackable, deliver: 5, slotsCap: 1, stackCap: 100, wantErr: domain.ErrStorageFull},
		{name: "non stackable uses slot check", stash: &fakeStashRepository{items: []domain.StashItem{{ID: 1}}}, snapshot: domain.StashItem{NameID: 1201, Refine: 7}, deliver: 1, slotsCap: 1, stackCap: 100, wantErr: domain.ErrStorageFull},
		{name: "free slot available", stash: &fakeStashRepository{}, snapshot: stackable, deliver: 5, slotsCap: 600, stackCap: 100, wantErr: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			service := buildOffer(&stubListings{}, tt.stash, &stubEscrow{}, &stubWallet{}, &stubSettlement{}, WithCapacity(tt.slotsCap, tt.stackCap))

			err := service.ensureCapacity(context.Background(), 1, tt.snapshot, tt.deliver)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ensureCapacity() err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestOfferService_Create_HoldFailure(t *testing.T) {
	t.Parallel()

	wallet := &stubWallet{holdErr: errOfferTest}
	service := buildOffer(&stubListings{}, &fakeStashRepository{}, &stubEscrow{}, wallet, &stubSettlement{})

	_, err := service.Create(context.Background(), CreateInput{
		Kind: domain.KindBuy, WantNameID: 501, WantUnitAmount: 1, GiveZeny: 500, Quantity: 1,
	})
	if !errors.Is(err, errOfferTest) {
		t.Errorf("Create() err = %v, want propagated hold error", err)
	}
}

func TestOfferService_Cancel_GetFailure(t *testing.T) {
	t.Parallel()

	service := buildOffer(&stubListings{getErr: errOfferTest}, &fakeStashRepository{}, &stubEscrow{}, &stubWallet{}, &stubSettlement{})

	err := service.Cancel(context.Background(), 1, 5)
	if !errors.Is(err, errOfferTest) {
		t.Errorf("Cancel() err = %v, want propagated get error", err)
	}
}

func TestOfferService_Cancel_ReleaseFailure(t *testing.T) {
	t.Parallel()

	holdID := int64(55)
	listings := &stubListings{getListing: domain.Listing{ID: 9, SellerAccountID: 5, Status: domain.StatusActive, GiveHoldID: &holdID}}
	wallet := &stubWallet{releaseErr: errOfferTest}
	service := buildOffer(listings, &fakeStashRepository{}, &stubEscrow{}, wallet, &stubSettlement{})

	err := service.Cancel(context.Background(), 9, 5)
	if !errors.Is(err, errOfferTest) {
		t.Errorf("Cancel() err = %v, want propagated release error", err)
	}
}
