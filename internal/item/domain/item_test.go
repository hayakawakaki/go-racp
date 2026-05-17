package domain

import (
	"slices"
	"testing"
)

func TestItemType_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
		in   ItemType
	}{
		{name: "healing", in: ItemTypeHealing, want: "Healing"},
		{name: "weapon", in: ItemTypeWeapon, want: "Weapon"},
		{name: "shadow gear", in: ItemTypeShadowGear, want: "ShadowGear"},
		{name: "cash", in: ItemTypeCash, want: "Cash"},
		{name: "unknown maps to empty", in: ItemTypeUnknown, want: ""},
		{name: "out of range maps to empty", in: ItemType(99), want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.in.String(); got != tt.want {
				t.Errorf("ItemType(%d).String() = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestItemType_Display(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
		in   ItemType
	}{
		{name: "healing renders as potion food", in: ItemTypeHealing, want: "Potion/Food"},
		{name: "pet egg renders with space", in: ItemTypePetEgg, want: "Pet Egg"},
		{name: "ammo renders friendly", in: ItemTypeAmmo, want: "Arrow/Ammunition"},
		{name: "delay consume renders friendly", in: ItemTypeDelayConsume, want: "Buff/Box"},
		{name: "unknown falls through", in: ItemTypeUnknown, want: "Unknown Type"},
		{name: "out of range falls through", in: ItemType(99), want: "Unknown Type"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.in.Display(); got != tt.want {
				t.Errorf("ItemType(%d).Display() = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestItemTypeFromString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		want   ItemType
		wantOK bool
	}{
		{name: "healing", input: "Healing", want: ItemTypeHealing, wantOK: true},
		{name: "weapon", input: "Weapon", want: ItemTypeWeapon, wantOK: true},
		{name: "cash", input: "Cash", want: ItemTypeCash, wantOK: true},
		{name: "case mismatch is not accepted", input: "healing", want: ItemTypeUnknown, wantOK: false},
		{name: "empty input", input: "", want: ItemTypeUnknown, wantOK: false},
		{name: "unknown name", input: "Mystery", want: ItemTypeUnknown, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := ItemTypeFromString(tt.input)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("ItemTypeFromString(%q) = (%v, %v), want (%v, %v)", tt.input, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestGenderFromString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  Gender
	}{
		{name: "both", input: "Both", want: GenderBoth},
		{name: "male", input: "Male", want: GenderMale},
		{name: "female", input: "Female", want: GenderFemale},
		{name: "empty falls back to both", input: "", want: GenderBoth},
		{name: "unknown falls back to both", input: "Other", want: GenderBoth},
		{name: "wrong case falls back to both", input: "male", want: GenderBoth},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := GenderFromString(tt.input); got != tt.want {
				t.Errorf("GenderFromString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestLocationFromString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		want   Location
		wantOK bool
	}{
		{name: "head top", input: "Head_Top", want: LocationHeadTop, wantOK: true},
		{name: "right hand", input: "Right_Hand", want: LocationRightHand, wantOK: true},
		{name: "both hand", input: "Both_Hand", want: LocationBothHand, wantOK: true},
		{name: "shadow right accessory", input: "Shadow_Right_Accessory", want: LocationShadowRightAccessory, wantOK: true},
		{name: "empty", input: "", want: LocationNone, wantOK: false},
		{name: "unknown", input: "Forehead", want: LocationNone, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := LocationFromString(tt.input)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("LocationFromString(%q) = (%v, %v), want (%v, %v)", tt.input, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestLocationSet_SetAndHas(t *testing.T) {
	t.Parallel()

	var set LocationSet
	if set.Has(LocationRightHand) {
		t.Errorf("empty set should not contain Right_Hand")
	}

	set.Set(LocationRightHand)
	if !set.Has(LocationRightHand) {
		t.Errorf("Set then Has = false, want true")
	}
	if set.Has(LocationLeftHand) {
		t.Errorf("Has(LeftHand) = true after only setting RightHand, want false")
	}

	set.Set(LocationLeftHand)
	if !set.Has(LocationLeftHand) || !set.Has(LocationRightHand) {
		t.Errorf("after Setting Left and Right, expected both Has to be true; got left=%v right=%v",
			set.Has(LocationLeftHand), set.Has(LocationRightHand))
	}
}

func TestLocationSet_SetIsIdempotent(t *testing.T) {
	t.Parallel()

	var set LocationSet
	set.Set(LocationArmor)
	before := set
	set.Set(LocationArmor)
	if set != before {
		t.Errorf("Set on already-set bit changed value: before=%b after=%b", before, set)
	}
}

func TestLocationSet_Display(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		want      []string
		locations []Location
	}{
		{name: "empty set yields no labels", locations: nil, want: nil},
		{name: "single location", locations: []Location{LocationRightHand}, want: []string{"Weapon"}},
		{
			name:      "multiple locations ordered by bit position",
			locations: []Location{LocationShoes, LocationHeadTop, LocationArmor},
			want:      []string{"Head Top", "Armor", "Shoes"},
		},
		{
			name:      "shadow gear cluster",
			locations: []Location{LocationShadowArmor, LocationShadowWeapon, LocationShadowShield},
			want:      []string{"Shadow Armor", "Shadow Weapon", "Shadow Shield"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var set LocationSet
			for _, location := range tt.locations {
				set.Set(location)
			}
			got := set.Display()
			if !slices.Equal(got, tt.want) {
				t.Errorf("Display() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestItem_SlotSuffix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
		item Item
	}{
		{name: "weapon with slots", item: Item{Type: ItemTypeWeapon, Slots: 3}, want: " [3]"},
		{name: "armor with slots", item: Item{Type: ItemTypeArmor, Slots: 1}, want: " [1]"},
		{name: "weapon zero slots emits nothing", item: Item{Type: ItemTypeWeapon, Slots: 0}, want: ""},
		{name: "card with slots emits nothing", item: Item{Type: ItemTypeCard, Slots: 4}, want: ""},
		{name: "healing with slots emits nothing", item: Item{Type: ItemTypeHealing, Slots: 2}, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.item.SlotSuffix(); got != tt.want {
				t.Errorf("SlotSuffix() = %q, want %q", got, tt.want)
			}
		})
	}
}
