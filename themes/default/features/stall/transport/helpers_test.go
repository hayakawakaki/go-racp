package transport

import (
	"testing"

	"github.com/hayakawakaki/go-racp/internal/features/stall/domain"
)

func TestItemIDValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
		in   int
	}{
		{name: "zero", in: 0, want: ""},
		{name: "negative", in: -1, want: ""},
		{name: "positive", in: 501, want: "501"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := itemIDValue(tt.in); got != tt.want {
				t.Errorf("got = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRowID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
		in   domain.Vendor
	}{
		{
			name: "selling",
			in:   domain.Vendor{ID: 42, Type: domain.VendorTypeSelling},
			want: "vendor-selling-42",
		},
		{
			name: "buying",
			in:   domain.Vendor{ID: 7, Type: domain.VendorTypeBuying},
			want: "vendor-buying-7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := rowID(tt.in); got != tt.want {
				t.Errorf("got = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTypeLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
		in   domain.VendorType
	}{
		{name: "selling", in: domain.VendorTypeSelling, want: "Selling"},
		{name: "buying", in: domain.VendorTypeBuying, want: "Buying"},
		{name: "unknown", in: domain.VendorTypeUnknown, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := typeLabel(tt.in); got != tt.want {
				t.Errorf("got = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTypeBadgeClass(t *testing.T) {
	t.Parallel()

	selling := typeBadgeClass(domain.VendorTypeSelling)
	buying := typeBadgeClass(domain.VendorTypeBuying)
	unknown := typeBadgeClass(domain.VendorTypeUnknown)

	if selling == "" || buying == "" || unknown == "" {
		t.Errorf("badge classes must be non-empty: selling=%q buying=%q unknown=%q", selling, buying, unknown)
	}
	if selling == buying || selling == unknown || buying == unknown {
		t.Errorf("badge classes must be distinct: selling=%q buying=%q unknown=%q", selling, buying, unknown)
	}
}
