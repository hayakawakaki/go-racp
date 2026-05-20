package domain

import (
	"testing"
	"time"
)

func TestWindowKey_Daily(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 20, 15, 42, 11, 7, time.UTC)
	got := WindowKey(WindowDaily, now)
	want := time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("Daily = %v, want %v", got, want)
	}
}

func TestWindowKey_Daily_PreservesLocation(t *testing.T) {
	t.Parallel()
	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Skipf("Asia/Tokyo unavailable: %v", err)
	}
	now := time.Date(2026, 5, 20, 8, 0, 0, 0, tokyo)
	got := WindowKey(WindowDaily, now)
	if got.Location().String() != tokyo.String() {
		t.Errorf("Location = %v, want %v", got.Location(), tokyo)
	}
	if y, m, d := got.Date(); y != 2026 || m != time.May || d != 20 {
		t.Errorf("Date = %d-%d-%d, want 2026-05-20", y, m, d)
	}
}

func TestWindowKey_Weekly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		now  time.Time
		want time.Time
		name string
	}{
		{
			name: "monday returns same day",
			now:  time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC),
			want: time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "wednesday rolls back to monday",
			now:  time.Date(2026, 5, 20, 23, 59, 59, 0, time.UTC),
			want: time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "sunday rolls back to prior monday (ISO week)",
			now:  time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC),
			want: time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "saturday rolls back to monday of same week",
			now:  time.Date(2026, 5, 23, 1, 0, 0, 0, time.UTC),
			want: time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := WindowKey(WindowWeekly, tt.now); !got.Equal(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWindowKey_Monthly(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 20, 15, 0, 0, 0, time.UTC)
	got := WindowKey(WindowMonthly, now)
	want := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("Monthly = %v, want %v", got, want)
	}
}

func TestWindowKey_AllTime(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 20, 15, 0, 0, 0, time.UTC)
	got := WindowKey(WindowAllTime, now)
	want := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("AllTime = %v, want sentinel %v", got, want)
	}
}

func TestWindowKey_Unknown_ReturnsSentinel(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)
	got := WindowKey(Window("bogus"), now)
	want := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("Unknown window = %v, want sentinel %v", got, want)
	}
}

func TestWindowKey_Daily_WeeklyAcrossMonthBoundary(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 3, 9, 0, 0, 0, time.UTC)
	got := WindowKey(WindowWeekly, now)
	want := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v (monday of week crossing month)", got, want)
	}
}
