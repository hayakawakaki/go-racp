package domain

import (
	"testing"
	"time"
)

func TestNotification_IsRead(t *testing.T) {
	t.Parallel()

	read := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		readAt *time.Time
		name   string
		want   bool
	}{
		{nil, "unread when ReadAt nil", false},
		{&read, "read when ReadAt set", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			n := Notification{ReadAt: tt.readAt}
			if got := n.IsRead(); got != tt.want {
				t.Errorf("IsRead() = %v, want %v", got, tt.want)
			}
		})
	}
}
