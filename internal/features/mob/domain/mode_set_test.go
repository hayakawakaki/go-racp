package domain

import (
	"slices"
	"testing"
)

func TestModeSet_HasSet(t *testing.T) {
	t.Parallel()

	var set ModeSet
	if set.Has(ModeMvp) {
		t.Fatal("zero ModeSet should not contain ModeMvp")
	}
	set.Set(ModeMvp)
	if !set.Has(ModeMvp) {
		t.Errorf("after Set(ModeMvp), Has(ModeMvp) = false, want true")
	}
	if set.Has(ModeAggressive) {
		t.Errorf("setting ModeMvp leaked into ModeAggressive")
	}
	set.Set(ModeAggressive)
	if !set.Has(ModeMvp) || !set.Has(ModeAggressive) {
		t.Errorf("Set should accumulate flags, got Has(ModeMvp)=%v Has(ModeAggressive)=%v",
			set.Has(ModeMvp), set.Has(ModeAggressive))
	}
}

func TestModeSet_Display_EmptyReturnsNil(t *testing.T) {
	t.Parallel()

	var set ModeSet
	if got := set.Display(); got != nil {
		t.Errorf("empty set Display() = %v, want nil", got)
	}
}

func TestModeSet_Display_OrderedByEnumValue(t *testing.T) {
	t.Parallel()

	var set ModeSet
	set.Set(ModeMvp)
	set.Set(ModeAggressive)
	set.Set(ModeCanMove)

	got := set.Display()
	want := []string{"Can Move", "Aggressive", "MVP"}
	if !slices.Equal(got, want) {
		t.Errorf("Display() = %v, want %v", got, want)
	}
}

func TestModesFromMap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input map[string]bool
		want  []string
	}{
		{name: "nil map", input: nil, want: nil},
		{name: "empty map", input: map[string]bool{}, want: nil},
		{
			name:  "single mode",
			input: map[string]bool{"Aggressive": true},
			want:  []string{"Aggressive"},
		},
		{
			name:  "false entries are ignored",
			input: map[string]bool{"Aggressive": false, "Mvp": true},
			want:  []string{"MVP"},
		},
		{
			name:  "unknown name silently dropped",
			input: map[string]bool{"NotARealMode": true, "Looter": true},
			want:  []string{"Looter"},
		},
		{
			name:  "multiple modes",
			input: map[string]bool{"CanMove": true, "Aggressive": true, "Mvp": true},
			want:  []string{"Can Move", "Aggressive", "MVP"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ModesFromMap(tt.input).Display()
			if !slices.Equal(got, tt.want) {
				t.Errorf("ModesFromMap(%v).Display() = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
