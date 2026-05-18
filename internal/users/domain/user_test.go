package domain

import "testing"

func TestUser_IsAdmin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		groupID int
		want    bool
	}{
		{"player", 0, false},
		{"moderator", 20, false},
		{"admin", 99, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			u := &User{GroupID: tt.groupID}
			if got := u.IsAdmin(); got != tt.want {
				t.Errorf("IsAdmin() = %v, want %v", got, tt.want)
			}
		})
	}
}
