package domain

import (
	"testing"
	"time"
)

func TestSession_IsExpired(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		now       time.Time
		expiresAt time.Time
		name      string
		want      bool
	}{
		{name: "future expiry", now: base, expiresAt: base.Add(time.Minute), want: false},
		{name: "exact boundary counts as expired", now: base, expiresAt: base, want: true},
		{name: "past expiry", now: base.Add(time.Second), expiresAt: base, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := &Session{ExpiresAt: tt.expiresAt}
			if got := s.IsExpired(tt.now); got != tt.want {
				t.Errorf("IsExpired(%v) with ExpiresAt=%v = %v, want %v",
					tt.now, tt.expiresAt, got, tt.want)
			}
		})
	}
}
