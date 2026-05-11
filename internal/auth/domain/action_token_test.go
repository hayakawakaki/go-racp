package domain

import (
	"database/sql"
	"testing"
	"time"
)

func TestActionToken_IsExpired(t *testing.T) {
	t.Parallel()

	expires := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		now     time.Time
		expires time.Time
		name    string
		want    bool
	}{
		{
			name:    "before expiry",
			now:     expires.Add(-time.Second),
			expires: expires,
			want:    false,
		},
		{
			name:    "exactly at expiry",
			now:     expires,
			expires: expires,
			want:    true,
		},
		{
			name:    "after expiry",
			now:     expires.Add(time.Second),
			expires: expires,
			want:    true,
		},
		{
			name:    "well before expiry",
			now:     expires.Add(-24 * time.Hour),
			expires: expires,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			token := &ActionToken{ExpiresAt: tt.expires}
			if got := token.IsExpired(tt.now); got != tt.want {
				t.Errorf("IsExpired(%v) with ExpiresAt=%v = %v, want %v", tt.now, tt.expires, got, tt.want)
			}
		})
	}
}

func TestActionToken_IsConsumed(t *testing.T) {
	t.Parallel()

	when := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		consumed sql.NullTime
		name     string
		want     bool
	}{
		{
			name:     "zero NullTime",
			consumed: sql.NullTime{},
			want:     false,
		},
		{
			name:     "valid timestamp",
			consumed: sql.NullTime{Time: when, Valid: true},
			want:     true,
		},
		{
			name:     "invalid even with time set",
			consumed: sql.NullTime{Time: when, Valid: false},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			token := &ActionToken{ConsumedAt: tt.consumed}
			if got := token.IsConsumed(); got != tt.want {
				t.Errorf("IsConsumed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAction_EnumValues(t *testing.T) {
	t.Parallel()

	if ActionUnknown != 0 {
		t.Errorf("ActionUnknown = %d, want 0 (sentinel at iota 0)", ActionUnknown)
	}
	if ActionEmailVerification != 1 {
		t.Errorf("ActionEmailVerification = %d, want 1", ActionEmailVerification)
	}
	if ActionUnknown == ActionEmailVerification {
		t.Errorf("ActionUnknown and ActionEmailVerification must be distinct")
	}
}
