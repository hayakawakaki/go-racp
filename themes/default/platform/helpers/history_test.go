package helpers

import (
	"testing"
	"time"
)

func TestFormatTime(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	plusOne := time.FixedZone("UTC+1", 3600)

	tests := []struct {
		loc  *time.Location
		name string
		want string
	}{
		{name: "nil location falls back to utc", loc: nil, want: "2026-05-29 12:00"},
		{name: "explicit utc", loc: time.UTC, want: "2026-05-29 12:00"},
		{name: "offset location", loc: plusOne, want: "2026-05-29 13:00"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := FormatTime(base, tt.loc); got != tt.want {
				t.Errorf("FormatTime(%v, %v) = %q, want %q", base, tt.loc, got, tt.want)
			}
		})
	}
}

func TestFormatSentTime(t *testing.T) {
	t.Parallel()

	sent := time.Date(2026, 5, 29, 8, 30, 0, 0, time.UTC)

	tests := []struct {
		in   *time.Time
		name string
		want string
	}{
		{name: "nil is not sent", in: nil, want: "not sent"},
		{name: "set formats in location", in: &sent, want: "2026-05-29 08:30"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := FormatSentTime(tt.in, time.UTC); got != tt.want {
				t.Errorf("FormatSentTime(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestWithdrawStatusLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		want   string
		status int
	}{
		{name: "sent", status: 2, want: "Sent"},
		{name: "pending", status: 1, want: "Pending"},
		{name: "zero is pending", status: 0, want: "Pending"},
		{name: "delivered", status: 3, want: "Delivered"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := WithdrawStatusLabel(tt.status); got != tt.want {
				t.Errorf("WithdrawStatusLabel(%d) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestHistoryHref(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		basePath     string
		primaryKey   string
		primaryValue string
		otherKey     string
		want         string
		otherPage    int
	}{
		{
			name:         "other page omitted when first",
			basePath:     "/admin/economy",
			primaryKey:   "dpage",
			primaryValue: "__PAGE__",
			otherKey:     "wpage",
			otherPage:    1,
			want:         "/admin/economy?dpage=__PAGE__",
		},
		{
			name:         "other page included when beyond first",
			basePath:     "/admin/economy",
			primaryKey:   "dpage",
			primaryValue: "__PAGE__",
			otherKey:     "wpage",
			otherPage:    3,
			want:         "/admin/economy?dpage=__PAGE__&wpage=3",
		},
		{
			name:         "withdraw primary keeps deposit page",
			basePath:     "/users/7",
			primaryKey:   "wpage",
			primaryValue: "__PAGE__",
			otherKey:     "dpage",
			otherPage:    2,
			want:         "/users/7?dpage=2&wpage=__PAGE__",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := HistoryHref(tt.basePath, tt.primaryKey, tt.primaryValue, tt.otherKey, tt.otherPage)
			if got != tt.want {
				t.Errorf("HistoryHref(%q, %q, %q, %q, %d) = %q, want %q", tt.basePath, tt.primaryKey, tt.primaryValue, tt.otherKey, tt.otherPage, got, tt.want)
			}
		})
	}
}
