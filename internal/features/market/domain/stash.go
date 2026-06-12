package domain

import (
	"context"
	"time"
)

const (
	DefaultMaxStorageSlots = 600
	DefaultMaxStackAmount  = 30000
)

type StashItem struct {
	ID         int64
	UniqueID   int64
	ExpireTime int64
	AccountID  int
	NameID     int
	Amount     int
	Equip      int
	Identify   int
	Refine     int
	Attribute  int
	Bound      int
	Grade      int
	Card       [4]int
	OptionID   [5]int
	OptionVal  [5]int
	OptionParm [5]int
}

func (i StashItem) IsTradable() bool {
	return i.Bound == 0 && i.ExpireTime == 0
}

func (i StashItem) IsStackable() bool {
	if i.Equip != 0 || i.Refine != 0 || i.Grade != 0 || i.Bound != 0 || i.Attribute != 0 || i.UniqueID != 0 {
		return false
	}

	return !i.hasCards() && !i.hasOptions()
}

func (i StashItem) hasCards() bool {
	for _, card := range i.Card {
		if card != 0 {
			return true
		}
	}

	return false
}

func (i StashItem) hasOptions() bool {
	for index := range i.OptionID {
		if i.OptionID[index] != 0 || i.OptionVal[index] != 0 || i.OptionParm[index] != 0 {
			return true
		}
	}

	return false
}

func (i StashItem) Mergeable(other StashItem) bool {
	return i.IsStackable() && other.IsStackable() &&
		i.NameID == other.NameID && i.Identify == other.Identify &&
		i.ExpireTime == other.ExpireTime
}

type EscrowMove struct {
	StashItemID int64
	Amount      int
}

type StashRepository interface {
	ListByAccount(ctx context.Context, accountID int) ([]StashItem, error)
	IsLocked(ctx context.Context, accountID int) (bool, error)
	SlotsUsed(ctx context.Context, accountID int) (int, error)
	MergeableAmount(ctx context.Context, accountID int, item StashItem) (existingAmount int, found bool, err error)
}

type EscrowRepository interface {
	MoveToEscrow(ctx context.Context, accountID int, listingRef int64, moves []EscrowMove) error
	ReturnToStash(ctx context.Context, listingRef int64) error
	Deliver(ctx context.Context, listingRef int64, toAccountID int, legID int64) error
	DeliverPartial(ctx context.Context, listingRef int64, toAccountID, amount int, legID int64) error
	ByListing(ctx context.Context, listingRef int64) ([]StashItem, error)
	OrphanRefs(ctx context.Context, before time.Time) ([]int64, error)
}
