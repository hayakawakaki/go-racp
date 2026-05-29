package domain

import (
	"errors"
	"math"
	"testing"
)

func TestAddZeny(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		name    string
		current int64
		delta   int64
		want    int64
	}{
		{name: "adds", current: 100, delta: 50, want: 150},
		{name: "zero delta", current: 100, delta: 0, want: 100},
		{name: "negative delta", current: 100, delta: -1, wantErr: ErrInvalidAmount},
		{name: "max boundary", current: math.MaxInt64 - 1, delta: 1, want: math.MaxInt64},
		{name: "overflow", current: math.MaxInt64, delta: 1, wantErr: ErrAmountOverflow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := AddZeny(tt.current, tt.delta)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("AddZeny(%d, %d) err = %v, want %v", tt.current, tt.delta, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("AddZeny(%d, %d) unexpected err: %v", tt.current, tt.delta, err)
			}
			if got != tt.want {
				t.Errorf("AddZeny(%d, %d) = %d, want %d", tt.current, tt.delta, got, tt.want)
			}
		})
	}
}

func TestAddCashpoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		name    string
		current int
		delta   int
		want    int
	}{
		{name: "adds", current: 100, delta: 50, want: 150},
		{name: "zero delta", current: 100, delta: 0, want: 100},
		{name: "negative delta", current: 100, delta: -1, wantErr: ErrInvalidAmount},
		{name: "max boundary", current: math.MaxInt32 - 1, delta: 1, want: math.MaxInt32},
		{name: "overflow", current: math.MaxInt32, delta: 1, wantErr: ErrAmountOverflow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := AddCashpoint(tt.current, tt.delta)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("AddCashpoint(%d, %d) err = %v, want %v", tt.current, tt.delta, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("AddCashpoint(%d, %d) unexpected err: %v", tt.current, tt.delta, err)
			}
			if got != tt.want {
				t.Errorf("AddCashpoint(%d, %d) = %d, want %d", tt.current, tt.delta, got, tt.want)
			}
		})
	}
}

func TestAddZenyCapped(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		current int64
		delta   int64
		want    int64
	}{
		{name: "adds", current: 100, delta: 50, want: 150},
		{name: "zero delta", current: 100, delta: 0, want: 100},
		{name: "negative delta is no-op", current: 100, delta: -1, want: 100},
		{name: "max boundary", current: math.MaxInt64 - 1, delta: 1, want: math.MaxInt64},
		{name: "overflow saturates", current: math.MaxInt64, delta: 1, want: math.MaxInt64},
		{name: "large overflow saturates", current: math.MaxInt64 - 10, delta: 1000, want: math.MaxInt64},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := AddZenyCapped(tt.current, tt.delta); got != tt.want {
				t.Errorf("AddZenyCapped(%d, %d) = %d, want %d", tt.current, tt.delta, got, tt.want)
			}
		})
	}
}

func TestAddCashpointCapped(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		current int
		delta   int
		want    int
	}{
		{name: "adds", current: 100, delta: 50, want: 150},
		{name: "zero delta", current: 100, delta: 0, want: 100},
		{name: "negative delta is no-op", current: 100, delta: -1, want: 100},
		{name: "max boundary", current: math.MaxInt32 - 1, delta: 1, want: math.MaxInt32},
		{name: "overflow saturates", current: math.MaxInt32, delta: 1, want: math.MaxInt32},
		{name: "large overflow saturates", current: math.MaxInt32 - 10, delta: 1000, want: math.MaxInt32},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := AddCashpointCapped(tt.current, tt.delta); got != tt.want {
				t.Errorf("AddCashpointCapped(%d, %d) = %d, want %d", tt.current, tt.delta, got, tt.want)
			}
		})
	}
}

func TestSubZeny(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		name    string
		current int64
		delta   int64
		want    int64
	}{
		{name: "subtracts", current: 100, delta: 40, want: 60},
		{name: "to zero", current: 100, delta: 100, want: 0},
		{name: "negative delta", current: 100, delta: -1, wantErr: ErrInvalidAmount},
		{name: "insufficient", current: 100, delta: 101, wantErr: ErrInsufficientBalance},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := SubZeny(tt.current, tt.delta)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("SubZeny(%d, %d) err = %v, want %v", tt.current, tt.delta, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("SubZeny(%d, %d) unexpected err: %v", tt.current, tt.delta, err)
			}
			if got != tt.want {
				t.Errorf("SubZeny(%d, %d) = %d, want %d", tt.current, tt.delta, got, tt.want)
			}
		})
	}
}

func TestSubCashpoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		name    string
		current int
		delta   int
		want    int
	}{
		{name: "subtracts", current: 100, delta: 40, want: 60},
		{name: "to zero", current: 100, delta: 100, want: 0},
		{name: "negative delta", current: 100, delta: -1, wantErr: ErrInvalidAmount},
		{name: "insufficient", current: 100, delta: 101, wantErr: ErrInsufficientBalance},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := SubCashpoint(tt.current, tt.delta)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("SubCashpoint(%d, %d) err = %v, want %v", tt.current, tt.delta, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("SubCashpoint(%d, %d) unexpected err: %v", tt.current, tt.delta, err)
			}
			if got != tt.want {
				t.Errorf("SubCashpoint(%d, %d) = %d, want %d", tt.current, tt.delta, got, tt.want)
			}
		})
	}
}

func TestAddBalance(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr   error
		name      string
		curZeny   int64
		deltaZeny int64
		curCash   int
		deltaCash int
		wantZeny  int64
		wantCash  int
	}{
		{name: "both add", curZeny: 100, deltaZeny: 50, curCash: 10, deltaCash: 5, wantZeny: 150, wantCash: 15},
		{name: "zeny overflow", curZeny: math.MaxInt64, deltaZeny: 1, curCash: 10, deltaCash: 5, wantErr: ErrAmountOverflow},
		{name: "cashpoint overflow", curZeny: 100, deltaZeny: 50, curCash: math.MaxInt32, deltaCash: 1, wantErr: ErrAmountOverflow},
		{name: "negative", curZeny: 100, deltaZeny: -1, curCash: 10, deltaCash: 5, wantErr: ErrInvalidAmount},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotZeny, gotCash, err := AddBalance(tt.curZeny, tt.deltaZeny, tt.curCash, tt.deltaCash)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("AddBalance err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("AddBalance unexpected err: %v", err)
			}
			if gotZeny != tt.wantZeny || gotCash != tt.wantCash {
				t.Errorf("AddBalance = (%d, %d), want (%d, %d)", gotZeny, gotCash, tt.wantZeny, tt.wantCash)
			}
		})
	}
}

func TestSubBalance(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr   error
		name      string
		curZeny   int64
		deltaZeny int64
		curCash   int
		deltaCash int
		wantZeny  int64
		wantCash  int
	}{
		{name: "both sub", curZeny: 100, deltaZeny: 40, curCash: 10, deltaCash: 4, wantZeny: 60, wantCash: 6},
		{name: "zeny insufficient", curZeny: 100, deltaZeny: 101, curCash: 10, deltaCash: 4, wantErr: ErrInsufficientBalance},
		{name: "cashpoint insufficient", curZeny: 100, deltaZeny: 40, curCash: 10, deltaCash: 11, wantErr: ErrInsufficientBalance},
		{name: "negative", curZeny: 100, deltaZeny: -1, curCash: 10, deltaCash: 4, wantErr: ErrInvalidAmount},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotZeny, gotCash, err := SubBalance(tt.curZeny, tt.deltaZeny, tt.curCash, tt.deltaCash)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("SubBalance err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("SubBalance unexpected err: %v", err)
			}
			if gotZeny != tt.wantZeny || gotCash != tt.wantCash {
				t.Errorf("SubBalance = (%d, %d), want (%d, %d)", gotZeny, gotCash, tt.wantZeny, tt.wantCash)
			}
		})
	}
}
