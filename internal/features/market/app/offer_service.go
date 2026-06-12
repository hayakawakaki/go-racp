package app

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/market/domain"
	"github.com/jackc/pgx/v5"
)

type Blacklist interface {
	Blocked(nameID int) bool
}

type emptyBlacklist struct{}

func (emptyBlacklist) Blocked(int) bool { return false }

type txBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

type OfferService struct {
	listings   domain.ListingRepository
	stash      domain.StashRepository
	escrow     domain.EscrowRepository
	wallet     domain.WalletRepository
	settlement domain.SettlementRepository
	db         txBeginner
	blacklist  Blacklist
	logger     *slog.Logger
	now        func() time.Time
	fee        domain.FeePolicy
	listingTTL time.Duration
	slotsTotal int
	stackCap   int
}

type Option func(*OfferService)

func WithNow(fn func() time.Time) Option {
	return func(s *OfferService) {
		if fn != nil {
			s.now = fn
		}
	}
}

func WithFeePolicy(policy domain.FeePolicy) Option {
	return func(s *OfferService) { s.fee = policy }
}

func WithListingTTL(d time.Duration) Option {
	return func(s *OfferService) {
		if d > 0 {
			s.listingTTL = d
		}
	}
}

func WithBlacklist(list Blacklist) Option {
	return func(s *OfferService) {
		if list != nil {
			s.blacklist = list
		}
	}
}

func WithCapacity(slotsTotal, stackCap int) Option {
	return func(s *OfferService) {
		if slotsTotal > 0 {
			s.slotsTotal = slotsTotal
		}
		if stackCap > 0 {
			s.stackCap = stackCap
		}
	}
}

func NewOfferService(
	listings domain.ListingRepository,
	stash domain.StashRepository,
	escrow domain.EscrowRepository,
	wallet domain.WalletRepository,
	settlement domain.SettlementRepository,
	db txBeginner,
	logger *slog.Logger,
	opts ...Option,
) *OfferService {
	s := &OfferService{
		listings:   listings,
		stash:      stash,
		escrow:     escrow,
		wallet:     wallet,
		settlement: settlement,
		db:         db,
		blacklist:  emptyBlacklist{},
		logger:     logger,
		now:        time.Now,
		fee:        domain.DefaultFeePolicy(),
		listingTTL: 7 * 24 * time.Hour,
		slotsTotal: domain.DefaultMaxStorageSlots,
		stackCap:   domain.DefaultMaxStackAmount,
	}
	for _, opt := range opts {
		opt(s)
	}

	return s
}

type CreateInput struct {
	SellerAccountID int
	Kind            int
	GiveStashItemID int64
	GiveUnitAmount  int
	GiveZeny        int64
	GiveCashpoint   int
	WantNameID      int
	WantUnitAmount  int
	WantZeny        int64
	WantCashpoint   int
	Quantity        int
}

func (s *OfferService) Create(ctx context.Context, in CreateInput) (int64, error) {
	if err := s.validateCreate(in); err != nil {
		return 0, err
	}
	if in.WantNameID != 0 && s.blacklist.Blocked(in.WantNameID) {
		return 0, domain.ErrItemBlacklisted
	}

	listing := s.newListing(in)

	if in.Kind != domain.KindBuy {
		if err := s.prepareGiveItem(ctx, in, &listing); err != nil {
			return 0, err
		}
	}

	ref, err := s.listings.NextRef(ctx)
	if err != nil {
		return 0, fmt.Errorf("app.OfferService.Create ref: %w", err)
	}
	listing.ID = ref

	if err := s.applyGiveSide(ctx, in, &listing); err != nil {
		return 0, err
	}

	if err := s.listings.Create(ctx, listing); err != nil {
		return 0, fmt.Errorf("app.OfferService.Create persist: %w", err)
	}

	if err := s.wallet.Burn(ctx, in.SellerAccountID, s.fee.FlatZeny, 0); err != nil {
		s.logger.Warn("market: flat fee burn", "listing", ref, "err", err)
	}

	return ref, nil
}

func (s *OfferService) newListing(in CreateInput) domain.Listing {
	return domain.Listing{
		SellerAccountID:   in.SellerAccountID,
		Kind:              in.Kind,
		Status:            domain.StatusActive,
		GiveUnitAmount:    in.GiveUnitAmount,
		GiveZeny:          in.GiveZeny,
		GiveCashpoint:     in.GiveCashpoint,
		WantNameID:        in.WantNameID,
		WantUnitAmount:    in.WantUnitAmount,
		WantZeny:          in.WantZeny,
		WantCashpoint:     in.WantCashpoint,
		TotalQuantity:     in.Quantity,
		RemainingQuantity: in.Quantity,
		ExpiresAt:         s.now().Add(s.listingTTL),
	}
}

func (s *OfferService) prepareGiveItem(ctx context.Context, in CreateInput, listing *domain.Listing) error {
	locked, err := s.stash.IsLocked(ctx, in.SellerAccountID)
	if err != nil {
		return fmt.Errorf("app.OfferService.Create lock: %w", err)
	}
	if !locked {
		return domain.ErrStorageUnlocked
	}

	giveItem, err := s.findStashItem(ctx, in.SellerAccountID, in.GiveStashItemID)
	if err != nil {
		return err
	}
	if s.blacklist.Blocked(giveItem.NameID) {
		return domain.ErrItemBlacklisted
	}

	listing.GiveItem = true
	listing.GiveNameID = giveItem.NameID
	listing.GiveRefine = giveItem.Refine
	listing.GiveGrade = giveItem.Grade
	listing.GiveCard = giveItem.Card
	listing.Stackable = giveItem.IsStackable()
	if !listing.Stackable && in.Quantity != 1 {
		return domain.ErrInvalidOffer
	}

	return nil
}

func (s *OfferService) applyGiveSide(ctx context.Context, in CreateInput, listing *domain.Listing) error {
	if listing.GiveItem {
		totalGive := in.GiveUnitAmount * in.Quantity
		if err := s.escrow.MoveToEscrow(ctx, in.SellerAccountID, listing.ID, []domain.EscrowMove{{StashItemID: in.GiveStashItemID, Amount: totalGive}}); err != nil {
			return fmt.Errorf("app.OfferService.Create escrow: %w", err)
		}
	}

	if in.Kind == domain.KindBuy {
		holdZeny := in.GiveZeny * int64(in.Quantity)
		holdCashpoint := in.GiveCashpoint * in.Quantity
		holdID, err := s.wallet.Hold(ctx, in.SellerAccountID, holdZeny, holdCashpoint)
		if err != nil {
			return fmt.Errorf("app.OfferService.Create hold: %w", err)
		}
		listing.GiveHoldID = &holdID
	}

	return nil
}

func (s *OfferService) validateCreate(in CreateInput) error {
	if in.Kind != domain.KindSell && in.Kind != domain.KindBuy {
		return domain.ErrInvalidOffer
	}
	if in.Quantity <= 0 {
		return domain.ErrInvalidOffer
	}
	if !validAmountBounds(in) {
		return domain.ErrInvalidOffer
	}

	if in.Kind == domain.KindBuy {
		if !validBuyOffer(in) {
			return domain.ErrInvalidOffer
		}

		return nil
	}

	if !validSellOffer(in) {
		return domain.ErrInvalidOffer
	}

	return nil
}

func validAmountBounds(in CreateInput) bool {
	if !domain.ScaledZenyWithinCap(in.WantZeny, in.Quantity) || !domain.ScaledZenyWithinCap(in.GiveZeny, in.Quantity) {
		return false
	}
	if !domain.ScaledCashpointWithinCap(in.WantCashpoint, in.Quantity) || !domain.ScaledCashpointWithinCap(in.GiveCashpoint, in.Quantity) {
		return false
	}

	return domain.ScaledCashpointWithinCap(in.GiveUnitAmount, in.Quantity) && domain.ScaledCashpointWithinCap(in.WantUnitAmount, in.Quantity)
}

func validBuyOffer(in CreateInput) bool {
	if in.WantNameID <= 0 || in.WantUnitAmount <= 0 {
		return false
	}

	return in.GiveZeny > 0 || in.GiveCashpoint > 0
}

func validSellOffer(in CreateInput) bool {
	if in.GiveUnitAmount <= 0 {
		return false
	}

	return in.WantZeny > 0 || in.WantCashpoint > 0
}

func (s *OfferService) findStashItem(ctx context.Context, accountID int, stashItemID int64) (domain.StashItem, error) {
	items, err := s.stash.ListByAccount(ctx, accountID)
	if err != nil {
		return domain.StashItem{}, fmt.Errorf("app.OfferService.findStashItem: %w", err)
	}
	for _, item := range items {
		if item.ID == stashItemID {
			return item, nil
		}
	}

	return domain.StashItem{}, domain.ErrStashItemNotFound
}

type TakeInput struct {
	ListingID        int64
	TakerAccountID   int
	Units            int
	TakerStashItemID int64
}

func (s *OfferService) Take(ctx context.Context, in TakeInput) error {
	listing, err := s.listings.Get(ctx, in.ListingID)
	if err != nil {
		return fmt.Errorf("app.OfferService.Take get: %w", err)
	}
	if validateErr := validateTake(listing, in); validateErr != nil {
		return validateErr
	}

	wantEscrowRef, err := s.prepareTake(ctx, listing, in)
	if err != nil {
		return err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("app.OfferService.Take begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	takenListing, _, err := s.listings.TakeUnitsTx(ctx, tx, in.ListingID, in.Units)
	if err != nil {
		return fmt.Errorf("app.OfferService.Take units: %w", err)
	}

	if settleErr := s.settleCurrencyTx(ctx, tx, takenListing, in); settleErr != nil {
		return fmt.Errorf("app.OfferService.Take settle: %w", settleErr)
	}

	if enqueueErr := s.enqueueLegsTx(ctx, tx, listing, in, wantEscrowRef); enqueueErr != nil {
		return enqueueErr
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("app.OfferService.Take commit: %w", err)
	}

	return nil
}

func validateTake(listing domain.Listing, in TakeInput) error {
	if listing.Status != domain.StatusActive {
		return domain.ErrListingInactive
	}
	if listing.SellerAccountID == in.TakerAccountID {
		return domain.ErrSelfTrade
	}
	if in.Units <= 0 || in.Units > listing.RemainingQuantity {
		return domain.ErrInsufficientUnits
	}

	return nil
}

func (s *OfferService) prepareTake(ctx context.Context, listing domain.Listing, in TakeInput) (int64, error) {
	if listing.GiveItem {
		giveSnapshot := domain.StashItem{
			NameID: listing.GiveNameID,
			Refine: listing.GiveRefine,
			Grade:  listing.GiveGrade,
			Card:   listing.GiveCard,
			Amount: listing.GiveUnitAmount * in.Units,
		}
		if err := s.ensureCapacity(ctx, in.TakerAccountID, giveSnapshot, listing.GiveUnitAmount*in.Units); err != nil {
			return 0, err
		}
	}

	if listing.WantNameID == 0 {
		return 0, nil
	}

	takerLocked, err := s.stash.IsLocked(ctx, in.TakerAccountID)
	if err != nil {
		return 0, fmt.Errorf("app.OfferService.Take lock: %w", err)
	}
	if !takerLocked {
		return 0, domain.ErrStorageUnlocked
	}

	return s.escrowTakerWant(ctx, listing, in)
}

func (s *OfferService) escrowTakerWant(ctx context.Context, listing domain.Listing, in TakeInput) (int64, error) {
	item, err := s.findStashItem(ctx, in.TakerAccountID, in.TakerStashItemID)
	if err != nil {
		return 0, err
	}

	needed := listing.WantUnitAmount * in.Units
	if !listing.WantSpec().Matches(item, needed) {
		return 0, domain.ErrWantMismatch
	}

	wantSnapshot := domain.StashItem{
		NameID: item.NameID,
		Refine: item.Refine,
		Grade:  item.Grade,
		Card:   item.Card,
		Amount: needed,
	}
	if capErr := s.ensureCapacity(ctx, listing.SellerAccountID, wantSnapshot, needed); capErr != nil {
		return 0, capErr
	}

	ref, err := s.listings.NextRef(ctx)
	if err != nil {
		return 0, fmt.Errorf("app.OfferService.escrowTakerWant ref: %w", err)
	}

	if moveErr := s.escrow.MoveToEscrow(ctx, in.TakerAccountID, ref, []domain.EscrowMove{{StashItemID: in.TakerStashItemID, Amount: needed}}); moveErr != nil {
		return 0, fmt.Errorf("app.OfferService.escrowTakerWant escrow: %w", moveErr)
	}

	return ref, nil
}

func (s *OfferService) settleCurrencyTx(ctx context.Context, tx pgx.Tx, listing domain.Listing, in TakeInput) error {
	units := int64(in.Units)

	switch listing.Kind {
	case domain.KindSell:
		grossZeny := listing.WantZeny * units
		grossCashpoint := listing.WantCashpoint * in.Units
		if grossZeny == 0 && grossCashpoint == 0 {
			return nil
		}

		if err := s.wallet.ChargeTx(ctx, tx, in.TakerAccountID, listing.SellerAccountID,
			grossZeny, grossCashpoint, s.fee.NetZeny(grossZeny), s.fee.NetCashpoint(grossCashpoint)); err != nil {
			return fmt.Errorf("app.OfferService.settleCurrencyTx charge: %w", err)
		}

		return nil

	case domain.KindBuy:
		if listing.GiveHoldID == nil {
			return nil
		}
		grossZeny := listing.GiveZeny * units
		grossCashpoint := listing.GiveCashpoint * in.Units

		if err := s.wallet.SettleHoldPartialTx(ctx, tx, *listing.GiveHoldID, in.TakerAccountID,
			grossZeny, grossCashpoint, s.fee.NetZeny(grossZeny), s.fee.NetCashpoint(grossCashpoint)); err != nil {
			return fmt.Errorf("app.OfferService.settleCurrencyTx settle: %w", err)
		}

		return nil

	default:
		return domain.ErrInvalidOffer
	}
}

func (s *OfferService) ensureCapacity(ctx context.Context, accountID int, snapshot domain.StashItem, deliverAmount int) error {
	if snapshot.IsStackable() {
		existing, found, err := s.stash.MergeableAmount(ctx, accountID, snapshot)
		if err != nil {
			return fmt.Errorf("app.OfferService.ensureCapacity: %w", err)
		}
		if found && existing+deliverAmount <= s.stackCap {
			return nil
		}
	}

	used, err := s.stash.SlotsUsed(ctx, accountID)
	if err != nil {
		return fmt.Errorf("app.OfferService.ensureCapacity: %w", err)
	}
	if used >= s.slotsTotal {
		return domain.ErrStorageFull
	}

	return nil
}

func (s *OfferService) enqueueLegsTx(ctx context.Context, tx pgx.Tx, listing domain.Listing, in TakeInput, wantEscrowRef int64) error {
	if listing.GiveItem {
		if err := s.settlement.EnqueueTx(ctx, tx, domain.SettlementLeg{
			ListingID:          listing.ID,
			EscrowRef:          listing.ID,
			RecipientAccountID: in.TakerAccountID,
			DeliverAmount:      listing.GiveUnitAmount * in.Units,
			Whole:              !listing.Stackable,
		}); err != nil {
			return fmt.Errorf("app.OfferService.Take enqueue give: %w", err)
		}
	}

	if listing.WantNameID != 0 {
		if err := s.settlement.EnqueueTx(ctx, tx, domain.SettlementLeg{
			ListingID:          listing.ID,
			EscrowRef:          wantEscrowRef,
			RecipientAccountID: listing.SellerAccountID,
			DeliverAmount:      0,
			Whole:              true,
		}); err != nil {
			return fmt.Errorf("app.OfferService.Take enqueue want: %w", err)
		}
	}

	return nil
}

func (s *OfferService) Cancel(ctx context.Context, listingID int64, byAccountID int) error {
	listing, err := s.listings.Get(ctx, listingID)
	if err != nil {
		return fmt.Errorf("app.OfferService.Cancel get: %w", err)
	}
	if listing.SellerAccountID != byAccountID {
		return domain.ErrListingNotFound
	}
	if listing.Status != domain.StatusActive {
		return domain.ErrListingInactive
	}

	if err := s.listings.SetStatus(ctx, listingID, domain.StatusCancelled); err != nil {
		return fmt.Errorf("app.OfferService.Cancel status: %w", err)
	}

	if listing.GiveItem {
		if err := s.settlement.Enqueue(ctx, domain.SettlementLeg{
			ListingID:          listing.ID,
			EscrowRef:          listing.ID,
			RecipientAccountID: listing.SellerAccountID,
			DeliverAmount:      listing.GiveUnitAmount * listing.RemainingQuantity,
			Whole:              !listing.Stackable,
		}); err != nil {
			return fmt.Errorf("app.OfferService.Cancel enqueue: %w", err)
		}
	}

	if listing.GiveHoldID != nil {
		if err := s.wallet.Release(ctx, *listing.GiveHoldID); err != nil {
			return fmt.Errorf("app.OfferService.Cancel release: %w", err)
		}
	}

	return nil
}

func (s *OfferService) Browse(ctx context.Context, kind, limit, offset int) ([]domain.Listing, int, error) {
	listings, total, err := s.listings.Browse(ctx, kind, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("app.OfferService.Browse: %w", err)
	}

	return listings, total, nil
}

func (s *OfferService) BySeller(ctx context.Context, accountID, limit, offset int) ([]domain.Listing, int, error) {
	listings, total, err := s.listings.BySeller(ctx, accountID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("app.OfferService.BySeller: %w", err)
	}

	return listings, total, nil
}
