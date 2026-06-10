package app

import (
	"context"
	"fmt"

	"github.com/hayakawakaki/go-racp/internal/features/market/domain"
)

type StashView struct {
	Items      []domain.StashItem
	Locked     bool
	SlotsUsed  int
	SlotsTotal int
}

type StashService struct {
	stash      domain.StashRepository
	slotsTotal int
}

func NewStashService(stash domain.StashRepository, slotsTotal int) *StashService {
	if slotsTotal <= 0 {
		slotsTotal = domain.DefaultMaxStorageSlots
	}

	return &StashService{stash: stash, slotsTotal: slotsTotal}
}

func (s *StashService) View(ctx context.Context, accountID int) (StashView, error) {
	locked, err := s.stash.IsLocked(ctx, accountID)
	if err != nil {
		return StashView{}, fmt.Errorf("app.StashService.View: %w", err)
	}

	items, err := s.stash.ListByAccount(ctx, accountID)
	if err != nil {
		return StashView{}, fmt.Errorf("app.StashService.View: %w", err)
	}

	return StashView{
		Items:      items,
		Locked:     locked,
		SlotsUsed:  len(items),
		SlotsTotal: s.slotsTotal,
	}, nil
}
