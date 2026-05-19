package self

import (
	"testing"
	"time"
)

func TestClassifyTier(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	tests := []struct {
		unbanTime time.Time
		name      string
		state     int
		want      Tier
	}{
		{name: "active state, zero unban time", state: 0, unbanTime: time.Time{}, want: TierActive},
		{name: "active state, unban time in past treats as elapsed", state: 0, unbanTime: past, want: TierActive},
		{name: "active state, unban time in future is temp banned", state: 0, unbanTime: future, want: TierTempBanned},
		{name: "unverified state", state: 1, unbanTime: time.Time{}, want: TierUnverified},
		{name: "unverified state ignores unban time", state: 1, unbanTime: future, want: TierUnverified},
		{name: "perma banned state", state: 5, unbanTime: time.Time{}, want: TierPermaBanned},
		{name: "perma banned state ignores unban time", state: 5, unbanTime: future, want: TierPermaBanned},
		{name: "unknown state 2 defaults to active", state: 2, unbanTime: time.Time{}, want: TierActive},
		{name: "unknown state 7 defaults to active", state: 7, unbanTime: future, want: TierActive},
		{name: "negative state defaults to active", state: -1, unbanTime: time.Time{}, want: TierActive},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ClassifyTier(tt.state, tt.unbanTime, now)
			if got != tt.want {
				t.Errorf("ClassifyTier(state=%d, unban=%v, now=%v) = %v, want %v", tt.state, tt.unbanTime, now, got, tt.want)
			}
		})
	}
}

func TestTier_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
		tier Tier
	}{
		{name: "active", tier: TierActive, want: "active"},
		{name: "unverified", tier: TierUnverified, want: "unverified"},
		{name: "temp banned", tier: TierTempBanned, want: "temp_banned"},
		{name: "perma banned", tier: TierPermaBanned, want: "perma_banned"},
		{name: "deleted", tier: TierDeleted, want: "deleted"},
		{name: "unknown", tier: Tier(99), want: "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.tier.String(); got != tt.want {
				t.Errorf("Tier(%d).String() = %q, want %q", tt.tier, got, tt.want)
			}
		})
	}
}
