package app

import (
	"testing"
	"time"
)

func TestCharacterDTO_LookOnCooldown(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		until time.Time
		name  string
		want  bool
	}{
		{name: "zero until is never on cooldown", until: time.Time{}, want: false},
		{name: "now strictly before until", until: now.Add(time.Minute), want: true},
		{name: "now exactly at until", until: now, want: false},
		{name: "now after until", until: now.Add(-time.Minute), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			d := CharacterDTO{LookCDUntil: tt.until}
			if got := d.LookOnCooldown(now); got != tt.want {
				t.Errorf("LookOnCooldown(%v) with LookCDUntil=%v = %v, want %v", now, tt.until, got, tt.want)
			}
		})
	}
}

func TestCharacterDTO_LocationOnCooldown(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		until time.Time
		name  string
		want  bool
	}{
		{name: "zero until is never on cooldown", until: time.Time{}, want: false},
		{name: "now strictly before until", until: now.Add(time.Hour), want: true},
		{name: "now exactly at until", until: now, want: false},
		{name: "now after until", until: now.Add(-time.Hour), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			d := CharacterDTO{LocCDUntil: tt.until}
			if got := d.LocationOnCooldown(now); got != tt.want {
				t.Errorf("LocationOnCooldown(%v) with LocCDUntil=%v = %v, want %v", now, tt.until, got, tt.want)
			}
		})
	}
}

func TestCharacterDTO_LookOnCooldown_IgnoresLocationField(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	d := CharacterDTO{LocCDUntil: now.Add(time.Hour)}
	if d.LookOnCooldown(now) {
		t.Errorf("LookOnCooldown returned true when only LocCDUntil was set")
	}
}

func TestCharacterDTO_LocationOnCooldown_IgnoresLookField(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	d := CharacterDTO{LookCDUntil: now.Add(time.Hour)}
	if d.LocationOnCooldown(now) {
		t.Errorf("LocationOnCooldown returned true when only LookCDUntil was set")
	}
}

func TestCharacterDTO_ZenyFormatted(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
		zeny int
	}{
		{name: "zero", zeny: 0, want: "0z"},
		{name: "single digit", zeny: 7, want: "7z"},
		{name: "below thousand boundary", zeny: 999, want: "999z"},
		{name: "exact thousand", zeny: 1000, want: "1,000z"},
		{name: "four digit", zeny: 1234, want: "1,234z"},
		{name: "five digit", zeny: 12345, want: "12,345z"},
		{name: "six digit", zeny: 123456, want: "123,456z"},
		{name: "round million", zeny: 1000000, want: "1,000,000z"},
		{name: "seven digit irregular", zeny: 1234567, want: "1,234,567z"},
		{name: "billion range", zeny: 1234567890, want: "1,234,567,890z"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := CharacterDTO{Zeny: tt.zeny}.ZenyFormatted()
			if got != tt.want {
				t.Errorf("ZenyFormatted(%d) = %q, want %q", tt.zeny, got, tt.want)
			}
		})
	}
}
