package infra

import (
	"errors"
	"testing"

	"github.com/hayakawakaki/go-racp/internal/features/market/domain"
)

func TestValidateEscrowMove(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		name    string
		item    domain.StashItem
		move    domain.EscrowMove
	}{
		{
			name: "full move non stackable",
			item: domain.StashItem{ID: 1, NameID: 1201, Amount: 1, Refine: 5},
			move: domain.EscrowMove{StashItemID: 1, Amount: 1},
		},
		{
			name: "partial move stackable",
			item: domain.StashItem{ID: 2, NameID: 501, Amount: 10},
			move: domain.EscrowMove{StashItemID: 2, Amount: 4},
		},
		{
			name:    "bound item",
			item:    domain.StashItem{ID: 3, NameID: 501, Amount: 10, Bound: 1},
			move:    domain.EscrowMove{StashItemID: 3, Amount: 4},
			wantErr: domain.ErrNotTradable,
		},
		{
			name:    "rental item",
			item:    domain.StashItem{ID: 4, NameID: 501, Amount: 10, ExpireTime: 1700000000},
			move:    domain.EscrowMove{StashItemID: 4, Amount: 4},
			wantErr: domain.ErrNotTradable,
		},
		{
			name:    "partial move carded",
			item:    domain.StashItem{ID: 5, NameID: 1201, Amount: 2, Card: [4]int{4001, 0, 0, 0}},
			move:    domain.EscrowMove{StashItemID: 5, Amount: 1},
			wantErr: domain.ErrNonStackable,
		},
		{
			name:    "amount zero",
			item:    domain.StashItem{ID: 6, NameID: 501, Amount: 10},
			move:    domain.EscrowMove{StashItemID: 6, Amount: 0},
			wantErr: domain.ErrInsufficientStack,
		},
		{
			name:    "amount above item amount",
			item:    domain.StashItem{ID: 7, NameID: 501, Amount: 10},
			move:    domain.EscrowMove{StashItemID: 7, Amount: 11},
			wantErr: domain.ErrInsufficientStack,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateEscrowMove(tt.item, tt.move)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("validateEscrowMove() err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewEscrowRepository_Clamps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		maxSlots     int
		maxStack     int
		wantMaxSlots int
		wantMaxStack int
	}{
		{
			name:         "zero slots and stack",
			maxSlots:     0,
			maxStack:     0,
			wantMaxSlots: domain.DefaultMaxStorageSlots,
			wantMaxStack: domain.DefaultMaxStackAmount,
		},
		{
			name:         "stack above default clamps back",
			maxSlots:     100,
			maxStack:     domain.DefaultMaxStackAmount + 1,
			wantMaxSlots: 100,
			wantMaxStack: domain.DefaultMaxStackAmount,
		},
		{
			name:         "valid values pass through",
			maxSlots:     200,
			maxStack:     500,
			wantMaxSlots: 200,
			wantMaxStack: 500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := NewEscrowRepository(nil, tt.maxSlots, tt.maxStack)
			if repo.MaxSlots != tt.wantMaxSlots {
				t.Errorf("MaxSlots = %d, want %d", repo.MaxSlots, tt.wantMaxSlots)
			}
			if repo.MaxStack != tt.wantMaxStack {
				t.Errorf("MaxStack = %d, want %d", repo.MaxStack, tt.wantMaxStack)
			}
		})
	}
}
