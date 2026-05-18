package domain

import "testing"

func TestJobName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		want    string
		classID int
	}{
		{"Novice", 0},
		{"Swordman", 1},
		{"Mage", 2},
		{"class_99999", 99999},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			got := JobName(tt.classID)
			if got != tt.want {
				t.Errorf("JobName(%d) = %q, want %q", tt.classID, got, tt.want)
			}
		})
	}
}
