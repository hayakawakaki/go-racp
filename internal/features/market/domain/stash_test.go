package domain

import "testing"

func TestStashItem_IsTradable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		item StashItem
		want bool
	}{
		{name: "zero item", item: StashItem{}, want: true},
		{name: "bound", item: StashItem{Bound: 1}, want: false},
		{name: "rental", item: StashItem{ExpireTime: 1700000000}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.item.IsTradable(); got != tt.want {
				t.Errorf("IsTradable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStashItem_IsStackable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		item StashItem
		want bool
	}{
		{name: "plain", item: StashItem{NameID: 501, Amount: 5}, want: true},
		{name: "equip", item: StashItem{Equip: 1}, want: false},
		{name: "refine", item: StashItem{Refine: 1}, want: false},
		{name: "grade", item: StashItem{Grade: 1}, want: false},
		{name: "bound", item: StashItem{Bound: 1}, want: false},
		{name: "attribute", item: StashItem{Attribute: 1}, want: false},
		{name: "unique id", item: StashItem{UniqueID: 1}, want: false},
		{name: "card slot", item: StashItem{Card: [4]int{0, 4001, 0, 0}}, want: false},
		{name: "option id", item: StashItem{OptionID: [5]int{0, 0, 7, 0, 0}}, want: false},
		{name: "option val", item: StashItem{OptionVal: [5]int{0, 0, 0, 12, 0}}, want: false},
		{name: "option parm", item: StashItem{OptionParm: [5]int{0, 0, 0, 0, 3}}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.item.IsStackable(); got != tt.want {
				t.Errorf("IsStackable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStashItem_Mergeable(t *testing.T) {
	t.Parallel()

	base := StashItem{NameID: 501, Amount: 5}

	tests := []struct {
		name  string
		left  StashItem
		right StashItem
		want  bool
	}{
		{name: "same nameid plain", left: base, right: StashItem{NameID: 501, Amount: 3}, want: true},
		{name: "differing nameid", left: base, right: StashItem{NameID: 502, Amount: 3}, want: false},
		{name: "differing identify", left: base, right: StashItem{NameID: 501, Identify: 1}, want: false},
		{name: "differing expire time", left: base, right: StashItem{NameID: 501, ExpireTime: 1700000000}, want: false},
		{name: "non stackable right", left: base, right: StashItem{NameID: 501, Refine: 1}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.left.Mergeable(tt.right); got != tt.want {
				t.Errorf("Mergeable() = %v, want %v", got, tt.want)
			}
		})
	}
}
