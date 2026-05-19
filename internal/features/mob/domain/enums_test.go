package domain

import "testing"

func TestRaceFromString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		want   Race
		wantOK bool
	}{
		{name: "formless", input: "Formless", want: RaceFormless, wantOK: true},
		{name: "demihuman", input: "Demihuman", want: RaceDemihuman, wantOK: true},
		{name: "dragon", input: "Dragon", want: RaceDragon, wantOK: true},
		{name: "case mismatch rejected", input: "formless", want: RaceFormless, wantOK: false},
		{name: "empty rejected", input: "", want: RaceFormless, wantOK: false},
		{name: "unknown name rejected", input: "Alien", want: RaceFormless, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := RaceFromString(tt.input)
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Errorf("got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRace_Display(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
		in   Race
	}{
		{name: "formless", in: RaceFormless, want: "Formless"},
		{name: "demihuman", in: RaceDemihuman, want: "Demihuman"},
		{name: "dragon", in: RaceDragon, want: "Dragon"},
		{name: "out of range falls back", in: Race(99), want: "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.in.Display(); got != tt.want {
				t.Errorf("Display() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestElementFromString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		want   Element
		wantOK bool
	}{
		{name: "neutral", input: "Neutral", want: ElementNeutral, wantOK: true},
		{name: "fire", input: "Fire", want: ElementFire, wantOK: true},
		{name: "undead element", input: "Undead", want: ElementUndead, wantOK: true},
		{name: "case mismatch rejected", input: "fire", want: ElementNeutral, wantOK: false},
		{name: "empty rejected", input: "", want: ElementNeutral, wantOK: false},
		{name: "unknown name rejected", input: "Plasma", want: ElementNeutral, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := ElementFromString(tt.input)
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Errorf("got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestElement_Display(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
		in   Element
	}{
		{name: "neutral", in: ElementNeutral, want: "Neutral"},
		{name: "fire", in: ElementFire, want: "Fire"},
		{name: "undead", in: ElementUndead, want: "Undead"},
		{name: "out of range falls back", in: Element(99), want: "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.in.Display(); got != tt.want {
				t.Errorf("Display() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSizeFromString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		want   Size
		wantOK bool
	}{
		{name: "small", input: "Small", want: SizeSmall, wantOK: true},
		{name: "medium", input: "Medium", want: SizeMedium, wantOK: true},
		{name: "large", input: "Large", want: SizeLarge, wantOK: true},
		{name: "case mismatch rejected", input: "small", want: SizeSmall, wantOK: false},
		{name: "empty rejected", input: "", want: SizeSmall, wantOK: false},
		{name: "unknown rejected", input: "Tiny", want: SizeSmall, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := SizeFromString(tt.input)
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Errorf("got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSize_Display(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
		in   Size
	}{
		{name: "small", in: SizeSmall, want: "Small"},
		{name: "medium", in: SizeMedium, want: "Medium"},
		{name: "large", in: SizeLarge, want: "Large"},
		{name: "out of range falls back", in: Size(99), want: "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.in.Display(); got != tt.want {
				t.Errorf("Display() = %q, want %q", got, tt.want)
			}
		})
	}
}
