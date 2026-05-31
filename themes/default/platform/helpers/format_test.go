package helpers

import (
	"math"
	"testing"
)

func TestFormatAmount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
		in   int64
	}{
		{name: "zero", in: 0, want: "0"},
		{name: "under thousand", in: 999, want: "999"},
		{name: "exact thousand", in: 1000, want: "1,000"},
		{name: "four digits", in: 1234, want: "1,234"},
		{name: "seven digits", in: 1234567, want: "1,234,567"},
		{name: "ten digits", in: 2000000000, want: "2,000,000,000"},
		{name: "max int64", in: math.MaxInt64, want: "9,223,372,036,854,775,807"},
		{name: "negative passthrough", in: -1000, want: "-1000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := FormatAmount(tt.in); got != tt.want {
				t.Errorf("FormatAmount(%d) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFormatPrice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		currency string
		want     string
		in       int64
	}{
		{name: "usd", in: 20, currency: "USD", want: "$20 USD"},
		{name: "usd thousands", in: 1000, currency: "USD", want: "$1,000 USD"},
		{name: "eur", in: 20, currency: "EUR", want: "€20 EUR"},
		{name: "jpy", in: 120, currency: "JPY", want: "¥120 JPY"},
		{name: "lowercase code", in: 20, currency: "usd", want: "$20 USD"},
		{name: "unmapped currency", in: 20, currency: "GBP", want: "20 GBP"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := FormatPrice(tt.in, tt.currency); got != tt.want {
				t.Errorf("FormatPrice(%d, %q) = %q, want %q", tt.in, tt.currency, got, tt.want)
			}
		})
	}
}
