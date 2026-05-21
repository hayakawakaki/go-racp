package domain

import (
	"testing"
	"time"
)

func TestLockoutPolicy_Backoff(t *testing.T) {
	t.Parallel()

	policy := DefaultLockoutPolicy()

	tests := []struct {
		name     string
		failures int
		want     time.Duration
	}{
		{name: "zero failures", failures: 0, want: 0},
		{name: "below threshold", failures: 4, want: 0},
		{name: "at threshold", failures: 5, want: 30 * time.Second},
		{name: "threshold plus one doubles", failures: 6, want: 60 * time.Second},
		{name: "threshold plus two doubles again", failures: 7, want: 120 * time.Second},
		{name: "threshold plus three", failures: 8, want: 240 * time.Second},
		{name: "shift saturates at max backoff", failures: 11, want: 30 * time.Minute},
		{name: "well above clamp", failures: 100, want: 30 * time.Minute},
		{name: "extreme value never overflows", failures: 1_000_000, want: 30 * time.Minute},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := policy.Backoff(tt.failures)
			if got != tt.want {
				t.Errorf("Backoff(%d) = %v, want %v", tt.failures, got, tt.want)
			}
		})
	}
}

func TestLockoutPolicy_Backoff_NeverNegative(t *testing.T) {
	t.Parallel()
	policy := DefaultLockoutPolicy()
	for failures := range 200 {
		if got := policy.Backoff(failures); got < 0 {
			t.Fatalf("Backoff(%d) = %v, must not be negative", failures, got)
		}
	}
}

func TestLockoutPolicy_Backoff_CapsAtMaxBackoff(t *testing.T) {
	t.Parallel()
	policy := LockoutPolicy{
		Window:      time.Hour,
		BaseBackoff: time.Second,
		MaxBackoff:  time.Minute,
		Threshold:   1,
	}
	for failures := 1; failures < 50; failures++ {
		if got := policy.Backoff(failures); got > policy.MaxBackoff {
			t.Fatalf("Backoff(%d) = %v, must not exceed MaxBackoff %v", failures, got, policy.MaxBackoff)
		}
	}
}

func TestDefaultLockoutPolicy_Values(t *testing.T) {
	t.Parallel()
	p := DefaultLockoutPolicy()
	if p.Window != 15*time.Minute {
		t.Errorf("Window = %v, want 15m", p.Window)
	}
	if p.Threshold != 5 {
		t.Errorf("Threshold = %d, want 5", p.Threshold)
	}
	if p.BaseBackoff != 30*time.Second {
		t.Errorf("BaseBackoff = %v, want 30s", p.BaseBackoff)
	}
	if p.MaxBackoff != 30*time.Minute {
		t.Errorf("MaxBackoff = %v, want 30m", p.MaxBackoff)
	}
}
