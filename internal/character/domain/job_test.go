package domain

import "testing"

func TestJobName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
		id   int
	}{
		{name: "novice at zero", id: 0, want: "Novice"},
		{name: "first class swordsman", id: 1, want: "Swordsman"},
		{name: "trans class lord knight", id: 4008, want: "Lord Knight"},
		{name: "third class dragon knight", id: 4252, want: "Dragon Knight"},
		{name: "fourth class boundary alitea", id: 4355, want: "Alitea"},
		{name: "gap inside known range", id: 13, want: "Unknown"},
		{name: "gap above first class block", id: 30, want: "Unknown"},
		{name: "negative id", id: -1, want: "Unknown"},
		{name: "far above max", id: 99999, want: "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := JobName(tt.id); got != tt.want {
				t.Errorf("JobName(%d) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}
