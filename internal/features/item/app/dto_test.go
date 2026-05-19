package app

import (
	"testing"

	"github.com/hayakawakaki/go-racp/internal/features/item/domain"
)

func TestToDTO_NilItemReturnsZeroValue(t *testing.T) {
	t.Parallel()

	got := ToDTO(nil)
	if got.ID != 0 || got.AegisName != "" || got.Name != "" || got.ClientName != "" ||
		got.Image != "" || got.Type != "" || got.SubType != "" || got.Weight != 0 ||
		got.Buy != 0 || got.Sell != 0 || got.Slots != 0 || got.Description != nil {
		t.Errorf("ToDTO(nil) = %+v, want zero ItemDTO", got)
	}
}

func TestToDTO_MapsAllFields(t *testing.T) {
	t.Parallel()

	item := &domain.Item{
		ID:          501,
		AegisName:   "Red_Potion",
		Name:        "Red Potion",
		ClientName:  "Red Potion",
		Image:       "red_potion",
		Type:        domain.ItemTypeHealing,
		SubType:     "",
		Description: []string{"line 1", "line 2"},
		Weight:      7.0,
		Buy:         10,
		Sell:        5,
		Slots:       0,
	}

	got := ToDTO(item)

	if got.ID != 501 {
		t.Errorf("ID = %d, want 501", got.ID)
	}
	if got.AegisName != "Red_Potion" {
		t.Errorf("AegisName = %q", got.AegisName)
	}
	if got.Name != "Red Potion" {
		t.Errorf("Name = %q", got.Name)
	}
	if got.ClientName != "Red Potion" {
		t.Errorf("ClientName = %q", got.ClientName)
	}
	if got.Image != "red_potion" {
		t.Errorf("Image = %q", got.Image)
	}
	if got.Type != "Healing" {
		t.Errorf("Type = %q, want %q", got.Type, "Healing")
	}
	if got.SubType != "" {
		t.Errorf("SubType = %q, want empty", got.SubType)
	}
	if len(got.Description) != 2 || got.Description[0] != "line 1" || got.Description[1] != "line 2" {
		t.Errorf("Description = %v", got.Description)
	}
	if got.Weight != 7.0 {
		t.Errorf("Weight = %v, want 7.0", got.Weight)
	}
	if got.Buy != 10 {
		t.Errorf("Buy = %d, want 10", got.Buy)
	}
	if got.Sell != 5 {
		t.Errorf("Sell = %d, want 5", got.Sell)
	}
	if got.Slots != 0 {
		t.Errorf("Slots = %d, want 0", got.Slots)
	}
}

func TestToDTO_WeaponPreservesSlotsAndSubType(t *testing.T) {
	t.Parallel()

	item := &domain.Item{
		ID:        1101,
		AegisName: "Sword",
		Name:      "Sword",
		Type:      domain.ItemTypeWeapon,
		SubType:   "1hSword",
		Slots:     3,
	}

	got := ToDTO(item)
	if got.Type != "Weapon" {
		t.Errorf("Type = %q, want %q", got.Type, "Weapon")
	}
	if got.SubType != "1hSword" {
		t.Errorf("SubType = %q, want %q", got.SubType, "1hSword")
	}
	if got.Slots != 3 {
		t.Errorf("Slots = %d, want 3", got.Slots)
	}
}

func TestToDTO_UnknownTypeRendersAsEmptyString(t *testing.T) {
	t.Parallel()

	item := &domain.Item{ID: 1, Type: domain.ItemTypeUnknown}
	got := ToDTO(item)
	if got.Type != "" {
		t.Errorf("Type = %q, want empty for ItemTypeUnknown", got.Type)
	}
}
