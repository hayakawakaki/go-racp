package domain

import "testing"

func TestVendorTypeFromString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input  string
		want   VendorType
		wantOk bool
	}{
		{input: "selling", want: VendorTypeSelling, wantOk: true},
		{input: "buying", want: VendorTypeBuying, wantOk: true},
		{input: "", want: VendorTypeUnknown, wantOk: false},
		{input: "all", want: VendorTypeUnknown, wantOk: false},
		{input: "Selling", want: VendorTypeUnknown, wantOk: false},
		{input: "bogus", want: VendorTypeUnknown, wantOk: false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got, ok := VendorTypeFromString(tt.input)
			if ok != tt.wantOk {
				t.Errorf("ok = %v, want %v", ok, tt.wantOk)
			}
			if got != tt.want {
				t.Errorf("got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVendorType_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
		in   VendorType
	}{
		{name: "selling", in: VendorTypeSelling, want: "selling"},
		{name: "buying", in: VendorTypeBuying, want: "buying"},
		{name: "unknown", in: VendorTypeUnknown, want: ""},
		{name: "out-of-range", in: VendorType(99), want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.in.String(); got != tt.want {
				t.Errorf("got = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVendorType_RoundTrip(t *testing.T) {
	t.Parallel()
	for _, vt := range []VendorType{VendorTypeSelling, VendorTypeBuying} {
		parsed, ok := VendorTypeFromString(vt.String())
		if !ok || parsed != vt {
			t.Errorf("round-trip %v -> %q -> (%v, %v)", vt, vt.String(), parsed, ok)
		}
	}
}

func TestVendor_Key(t *testing.T) {
	t.Parallel()
	v := Vendor{ID: 42, Type: VendorTypeSelling}
	got := v.Key()
	want := VendorKey{Type: VendorTypeSelling, ID: 42}
	if got != want {
		t.Errorf("got = %+v, want %+v", got, want)
	}
}
