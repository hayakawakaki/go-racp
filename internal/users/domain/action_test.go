package domain

import (
	"errors"
	"testing"
	"time"
)

func TestParseBanDuration_Presets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		preset string
		want   BanDuration
	}{
		{"1h", BanDuration{Duration: time.Hour}},
		{"1d", BanDuration{Duration: 24 * time.Hour}},
		{"7d", BanDuration{Duration: 7 * 24 * time.Hour}},
		{"30d", BanDuration{Duration: 30 * 24 * time.Hour}},
		{"perm", BanDuration{Permanent: true}},
	}
	for _, tt := range tests {
		t.Run(tt.preset, func(t *testing.T) {
			t.Parallel()
			got, err := ParseBanPreset(tt.preset)
			if err != nil {
				t.Fatalf("ParseBanPreset(%q): %v", tt.preset, err)
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseBanDuration_Custom(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err   error
		name  string
		unit  string
		want  time.Duration
		value int
	}{
		{nil, "3 hours", "hours", 3 * time.Hour, 3},
		{nil, "5 days", "days", 5 * 24 * time.Hour, 5},
		{ErrInvalidDuration, "zero", "hours", 0, 0},
		{ErrInvalidDuration, "negative", "days", 0, -1},
		{ErrInvalidDuration, "bad unit", "weeks", 0, 1},
		{ErrInvalidDuration, "over ceiling", "hours", 0, 24 * 366 * 11},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseBanCustom(tt.value, tt.unit)
			if tt.err != nil {
				if !errors.Is(err, tt.err) {
					t.Fatalf("err = %v, want %v", err, tt.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got.Duration != tt.want || got.Permanent {
				t.Errorf("got %+v, want Duration=%v", got, tt.want)
			}
		})
	}
}

func TestParseBanPreset_Unknown(t *testing.T) {
	t.Parallel()
	_, err := ParseBanPreset("forever")
	if !errors.Is(err, ErrInvalidDuration) {
		t.Errorf("err = %v, want ErrInvalidDuration", err)
	}
}
