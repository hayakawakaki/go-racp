package domain

import (
	"math"
	"testing"
)

func TestScaledZenyWithinCap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		unit     int64
		quantity int
		want     bool
	}{
		{name: "zero unit", unit: 0, quantity: 1000000, want: true},
		{name: "zero quantity", unit: 999999999, quantity: 0, want: true},
		{name: "negative unit", unit: -1, quantity: 1, want: false},
		{name: "negative quantity", unit: 1, quantity: -1, want: false},
		{name: "within cap", unit: 1000000, quantity: 1000, want: true},
		{name: "exactly cap", unit: MaxTransferZeny, quantity: 1, want: true},
		{name: "product over cap", unit: MaxTransferZeny, quantity: 2, want: false},
		{name: "large product over cap no overflow", unit: 1000000000, quantity: 2000, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ScaledZenyWithinCap(tt.unit, tt.quantity); got != tt.want {
				t.Errorf("ScaledZenyWithinCap(%d, %d) = %v, want %v", tt.unit, tt.quantity, got, tt.want)
			}
		})
	}
}

func TestScaledCashpointWithinCap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		unit     int
		quantity int
		want     bool
	}{
		{name: "zero unit", unit: 0, quantity: 5, want: true},
		{name: "zero quantity", unit: 5, quantity: 0, want: true},
		{name: "negative unit", unit: -1, quantity: 1, want: false},
		{name: "negative quantity", unit: 1, quantity: -1, want: false},
		{name: "within cap", unit: 1000, quantity: 1000, want: true},
		{name: "exactly cap", unit: math.MaxInt32, quantity: 1, want: true},
		{name: "product over cap", unit: math.MaxInt32, quantity: 2, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ScaledCashpointWithinCap(tt.unit, tt.quantity); got != tt.want {
				t.Errorf("ScaledCashpointWithinCap(%d, %d) = %v, want %v", tt.unit, tt.quantity, got, tt.want)
			}
		})
	}
}

func TestFeePolicy_NetZeny(t *testing.T) {
	t.Parallel()

	policy := DefaultFeePolicy()

	tests := []struct {
		name  string
		gross int64
		want  int64
	}{
		{name: "two percent fee", gross: 1000000, want: 980000},
		{name: "rounds fee down to zero on tiny amount", gross: 2, want: 2},
		{name: "zero", gross: 0, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := policy.NetZeny(tt.gross); got != tt.want {
				t.Errorf("NetZeny(%d) = %d, want %d", tt.gross, got, tt.want)
			}
		})
	}
}

func TestFeePolicy_NetCashpoint(t *testing.T) {
	t.Parallel()

	policy := DefaultFeePolicy()

	tests := []struct {
		name  string
		gross int
		want  int
	}{
		{name: "two percent fee", gross: 5000, want: 4900},
		{name: "rounds fee down to zero on tiny amount", gross: 3, want: 3},
		{name: "zero", gross: 0, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := policy.NetCashpoint(tt.gross); got != tt.want {
				t.Errorf("NetCashpoint(%d) = %d, want %d", tt.gross, got, tt.want)
			}
		})
	}
}

func TestWantSpec_Matches(t *testing.T) {
	t.Parallel()

	spec := WantSpec{NameID: 501, UnitAmount: 5}

	tests := []struct {
		name   string
		item   StashItem
		needed int
		want   bool
	}{
		{name: "nameid mismatch", item: StashItem{NameID: 502, Amount: 100}, needed: 5, want: false},
		{name: "amount below needed", item: StashItem{NameID: 501, Amount: 4}, needed: 5, want: false},
		{name: "exact amount", item: StashItem{NameID: 501, Amount: 5}, needed: 5, want: true},
		{name: "more than needed", item: StashItem{NameID: 501, Amount: 100}, needed: 5, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := spec.Matches(tt.item, tt.needed); got != tt.want {
				t.Errorf("Matches(%+v, %d) = %v, want %v", tt.item, tt.needed, got, tt.want)
			}
		})
	}
}
