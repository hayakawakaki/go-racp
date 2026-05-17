package domain

import "testing"

func TestJobsFromMap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input map[string]bool
		name  string
		want  JobMask
	}{
		{name: "nil input yields zero mask", input: nil, want: 0},
		{name: "empty map yields zero mask", input: map[string]bool{}, want: 0},
		{
			name:  "single enabled job",
			input: map[string]bool{"Swordman": true},
			want:  JobMask(1 << 25),
		},
		{
			name:  "disabled job is skipped",
			input: map[string]bool{"Swordman": false},
			want:  0,
		},
		{
			name:  "unknown name is skipped",
			input: map[string]bool{"Demigod": true},
			want:  0,
		},
		{
			name: "mix of enabled, disabled, and unknown",
			input: map[string]bool{
				"Swordman":  true,
				"Knight":    true,
				"Acolyte":   false,
				"Spaceman":  true,
				"Alchemist": true,
			},
			want: JobMask((1 << 25) | (1 << 11) | (1 << 2)),
		},
		{
			name:  "all sentinel takes its own bit",
			input: map[string]bool{"All": true},
			want:  JobMask(1 << 0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := JobsFromMap(tt.input); got != tt.want {
				t.Errorf("JobsFromMap(%v) = %b, want %b", tt.input, got, tt.want)
			}
		})
	}
}

func TestClassesFromMap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input map[string]bool
		name  string
		want  ClassMask
	}{
		{name: "nil input yields zero mask", input: nil, want: 0},
		{name: "empty map yields zero mask", input: map[string]bool{}, want: 0},
		{
			name:  "single class",
			input: map[string]bool{"Third": true},
			want:  ClassMask(1 << 4),
		},
		{
			name:  "disabled class is skipped",
			input: map[string]bool{"Third": false},
			want:  0,
		},
		{
			name:  "unknown class is skipped",
			input: map[string]bool{"Fifth": true},
			want:  0,
		},
		{
			name: "mix of enabled and unknown",
			input: map[string]bool{
				"Normal": true,
				"Upper":  true,
				"Fifth":  true,
				"Fourth": false,
				"Baby":   true,
			},
			want: ClassMask((1 << 1) | (1 << 2) | (1 << 3)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ClassesFromMap(tt.input); got != tt.want {
				t.Errorf("ClassesFromMap(%v) = %b, want %b", tt.input, got, tt.want)
			}
		})
	}
}
