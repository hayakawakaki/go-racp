package domain

import (
	"errors"
	"testing"
	"time"
)

func TestParseBanDays(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err  error
		name string
		want time.Duration
		days int
	}{
		{nil, "1 day", 24 * time.Hour, 1},
		{nil, "30 days", 30 * 24 * time.Hour, 30},
		{ErrInvalidDuration, "zero", 0, 0},
		{ErrInvalidDuration, "negative", 0, -1},
		{ErrInvalidDuration, "over ceiling", 0, 365 * 11},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseBanDays(tt.days)
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
