package domain

import (
	"testing"
	"time"
)

func TestAPIKey_IsRevoked(t *testing.T) {
	t.Parallel()

	revokedAt := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		revokedAt *time.Time
		name      string
		want      bool
	}{
		{name: "active key", revokedAt: nil, want: false},
		{name: "revoked key", revokedAt: &revokedAt, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			key := APIKey{RevokedAt: tt.revokedAt}
			if got := key.IsRevoked(); got != tt.want {
				t.Errorf("IsRevoked() = %v, want %v", got, tt.want)
			}
		})
	}
}
