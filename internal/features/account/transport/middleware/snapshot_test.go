package middleware

import (
	"context"
	"testing"

	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
)

func TestAccountSnapshot_IsAdmin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		groupID int
		want    bool
	}{
		{"player", 0, false},
		{"moderator", 20, false},
		{"enforcer", 10, false},
		{"event", 2, false},
		{"admin", domain.RoleAdmin.GroupID, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			snap := &AccountSnapshot{GroupID: tt.groupID}
			if got := snap.IsAdmin(); got != tt.want {
				t.Errorf("IsAdmin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSnapshotFromContext_RoundTrip(t *testing.T) {
	t.Parallel()

	want := &AccountSnapshot{UserID: 7, GroupID: 99, Username: "kaki"}
	ctx := ContextWithSnapshot(context.Background(), want)

	got, ok := SnapshotFromContext(ctx)
	if !ok {
		t.Fatal("SnapshotFromContext returned ok=false")
	}
	if got != want {
		t.Errorf("snapshot = %+v, want %+v", got, want)
	}
}

func TestSnapshotFromContext_AbsentReturnsFalse(t *testing.T) {
	t.Parallel()

	_, ok := SnapshotFromContext(context.Background())
	if ok {
		t.Error("SnapshotFromContext on empty context returned ok=true")
	}
}
