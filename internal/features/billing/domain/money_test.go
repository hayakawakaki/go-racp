package domain

import (
	"slices"
	"testing"
)

func TestToMinorUnits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		currency string
		amount   int64
		want     int64
		wantErr  bool
	}{
		{name: "usd whole units to cents", currency: "USD", amount: 5, want: 500},
		{name: "eur whole units to cents", currency: "EUR", amount: 20, want: 2000},
		{name: "jpy is zero decimal", currency: "JPY", amount: 500, want: 500},
		{name: "currency is case insensitive", currency: "usd", amount: 1, want: 100},
		{name: "unsupported currency errors", currency: "GBP", amount: 5, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ToMinorUnits(tt.amount, tt.currency)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ToMinorUnits(%d, %q) error = nil, want error", tt.amount, tt.currency)
				}

				return
			}
			if err != nil {
				t.Fatalf("ToMinorUnits(%d, %q): %v", tt.amount, tt.currency, err)
			}
			if got != tt.want {
				t.Errorf("ToMinorUnits(%d, %q) = %d, want %d", tt.amount, tt.currency, got, tt.want)
			}
		})
	}
}

func TestIsSupportedCurrency(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		currency string
		want     bool
	}{
		{name: "usd supported", currency: "USD", want: true},
		{name: "lowercase supported", currency: "jpy", want: true},
		{name: "gbp unsupported", currency: "GBP", want: false},
		{name: "empty unsupported", currency: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := IsSupportedCurrency(tt.currency); got != tt.want {
				t.Errorf("IsSupportedCurrency(%q) = %v, want %v", tt.currency, got, tt.want)
			}
		})
	}
}

func TestSupportedCurrencies(t *testing.T) {
	t.Parallel()

	got := SupportedCurrencies()
	want := []string{"EUR", "JPY", "USD"}
	if !slices.Equal(got, want) {
		t.Errorf("SupportedCurrencies() = %v, want %v", got, want)
	}
}
