package domain

import (
	"context"
	"math"
	"time"
)

const (
	KindSell = 1
	KindBuy  = 2
)

const (
	StatusActive    = 1
	StatusTaken     = 2
	StatusCancelled = 3
	StatusExpired   = 4
)

const (
	DefaultFeeBps      = 200
	DefaultFlatFeeZeny = 1000
)

type WantSpec struct {
	NameID     int
	UnitAmount int
}

func (s WantSpec) Matches(item StashItem, neededAmount int) bool {
	if s.NameID != item.NameID {
		return false
	}

	return item.Amount >= neededAmount
}

type Listing struct {
	CreatedAt         time.Time
	ExpiresAt         time.Time
	GiveHoldID        *int64
	ID                int64
	GiveZeny          int64
	WantZeny          int64
	SellerAccountID   int
	Kind              int
	Status            int
	GiveNameID        int
	GiveRefine        int
	GiveGrade         int
	GiveUnitAmount    int
	GiveCashpoint     int
	WantNameID        int
	WantUnitAmount    int
	WantCashpoint     int
	TotalQuantity     int
	RemainingQuantity int
	GiveCard          [4]int
	GiveItem          bool
	Stackable         bool
}

func (l Listing) WantSpec() WantSpec {
	return WantSpec{NameID: l.WantNameID, UnitAmount: l.WantUnitAmount}
}

type FeePolicy struct {
	Bps      int
	FlatZeny int64
}

func DefaultFeePolicy() FeePolicy {
	return FeePolicy{Bps: DefaultFeeBps, FlatZeny: DefaultFlatFeeZeny}
}

func (p FeePolicy) NetZeny(gross int64) int64 {
	return gross - gross*int64(p.Bps)/10000
}

func (p FeePolicy) NetCashpoint(gross int) int {
	return gross - gross*p.Bps/10000
}

func ScaledZenyWithinCap(unit int64, quantity int) bool {
	if unit < 0 || quantity < 0 {
		return false
	}
	if unit == 0 || quantity == 0 {
		return true
	}

	return int64(quantity) <= MaxTransferZeny/unit
}

func ScaledCashpointWithinCap(unit, quantity int) bool {
	if unit < 0 || quantity < 0 {
		return false
	}
	if unit == 0 || quantity == 0 {
		return true
	}

	return quantity <= math.MaxInt32/unit
}

type ListingRepository interface {
	NextRef(ctx context.Context) (int64, error)
	Create(ctx context.Context, listing Listing) error
	Get(ctx context.Context, id int64) (Listing, error)
	Browse(ctx context.Context, kind, limit, offset int) ([]Listing, int, error)
	BySeller(ctx context.Context, accountID, limit, offset int) ([]Listing, int, error)
	TakeUnits(ctx context.Context, id int64, units int) (Listing, bool, error)
	TakeUnitsTx(ctx context.Context, q DBTX, id int64, units int) (Listing, bool, error)
	SetStatus(ctx context.Context, id int64, status int) error
	DueForExpiry(ctx context.Context, now time.Time, limit int) ([]Listing, error)
	AllRefs(ctx context.Context) ([]int64, error)
}

type SettlementLeg struct {
	ID                 int64
	ListingID          int64
	EscrowRef          int64
	RecipientAccountID int
	DeliverAmount      int
	Whole              bool
}

type SettlementRepository interface {
	Enqueue(ctx context.Context, leg SettlementLeg) error
	EnqueueTx(ctx context.Context, q DBTX, leg SettlementLeg) error
	Pending(ctx context.Context, limit int) ([]SettlementLeg, error)
	PendingRefs(ctx context.Context) ([]int64, error)
	MarkDone(ctx context.Context, id int64) error
}
