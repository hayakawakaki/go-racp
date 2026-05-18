package httpx

import "testing"

func TestParsePositiveInt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		fallback int
		want     int
	}{
		{name: "empty uses fallback", input: "", fallback: 5, want: 5},
		{name: "valid positive integer", input: "12", fallback: 1, want: 12},
		{name: "zero falls back", input: "0", fallback: 1, want: 1},
		{name: "negative falls back", input: "-3", fallback: 1, want: 1},
		{name: "non-numeric falls back", input: "abc", fallback: 1, want: 1},
		{name: "one is accepted", input: "1", fallback: 7, want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ParsePositiveInt(tt.input, tt.fallback); got != tt.want {
				t.Errorf("ParsePositiveInt(%q, %d) = %d, want %d", tt.input, tt.fallback, got, tt.want)
			}
		})
	}
}
