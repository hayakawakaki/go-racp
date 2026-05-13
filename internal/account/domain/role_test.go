package domain

import (
	"testing"

	"github.com/hayakawakaki/go-racp/server/config"
)

func TestRole_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
		role Role
	}{
		{name: "any", role: RoleAny, want: "any"},
		{name: "player", role: RolePlayer, want: "player"},
		{name: "event", role: RoleEvent, want: "event"},
		{name: "moderator", role: RoleModerator, want: "moderator"},
		{name: "enforcer", role: RoleEnforcer, want: "enforcer"},
		{name: "admin", role: RoleAdmin, want: "admin"},
		{name: "unknown role value", role: Role(99), want: "unknown"},
		{name: "negative unknown", role: Role(-2), want: "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.role.String(); got != tt.want {
				t.Errorf("Role(%d).String() = %q, want %q", tt.role, got, tt.want)
			}
		})
	}
}

func TestRole_AtLeast(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		role    Role
		minimum Role
		want    bool
	}{
		{name: "admin satisfies admin", role: RoleAdmin, minimum: RoleAdmin, want: true},
		{name: "admin satisfies moderator", role: RoleAdmin, minimum: RoleModerator, want: true},
		{name: "moderator equals moderator", role: RoleModerator, minimum: RoleModerator, want: true},
		{name: "player below moderator", role: RolePlayer, minimum: RoleModerator, want: false},
		{name: "event below moderator", role: RoleEvent, minimum: RoleModerator, want: false},
		{name: "enforcer above moderator", role: RoleEnforcer, minimum: RoleModerator, want: true},
		{name: "any below player", role: RoleAny, minimum: RolePlayer, want: false},
		{name: "any below admin", role: RoleAny, minimum: RoleAdmin, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.role.AtLeast(tt.minimum); got != tt.want {
				t.Errorf("%v.AtLeast(%v) = %v, want %v", tt.role, tt.minimum, got, tt.want)
			}
		})
	}
}

func TestRoleResolver_Resolve(t *testing.T) {
	t.Parallel()

	resolver := NewRoleResolver(config.GroupConfig{
		Moderator: 20,
		Enforcer:  10,
		Event:     2,
	})

	tests := []struct {
		name    string
		groupID int
		want    Role
	}{
		{name: "hardcoded 99 is admin", groupID: 99, want: RoleAdmin},
		{name: "configured enforcer", groupID: 10, want: RoleEnforcer},
		{name: "configured moderator", groupID: 20, want: RoleModerator},
		{name: "configured event", groupID: 2, want: RoleEvent},
		{name: "default zero is player", groupID: 0, want: RolePlayer},
		{name: "unmapped positive is player", groupID: 50, want: RolePlayer},
		{name: "unmapped negative is player", groupID: -1, want: RolePlayer},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := resolver.Resolve(tt.groupID); got != tt.want {
				t.Errorf("Resolve(%d) = %v, want %v", tt.groupID, got, tt.want)
			}
		})
	}
}
