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
		{name: "authenticated sentinel", role: RoleAuthenticated, want: "*"},
		{name: "public sentinel", role: RolePublic, want: "Public"},
		{name: "player default", role: RolePlayer, want: "Player"},
		{name: "admin floor", role: RoleAdmin, want: "Admin"},
		{name: "custom role", role: Role{Name: "Moderator", GroupID: 20}, want: "Moderator"},
		{name: "empty role becomes unknown", role: Role{}, want: "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.role.String(); got != tt.want {
				t.Errorf("Role.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRoleResolver_Resolve(t *testing.T) {
	t.Parallel()

	resolver := NewRoleResolver(config.RolesConfig{
		"Moderator": 20,
		"Enforcer":  10,
		"Event":     2,
	})

	tests := []struct {
		name    string
		want    Role
		groupID int
	}{
		{name: "hardcoded 99 is admin", groupID: 99, want: RoleAdmin},
		{name: "configured enforcer", groupID: 10, want: Role{Name: "Enforcer", GroupID: 10}},
		{name: "configured moderator", groupID: 20, want: Role{Name: "Moderator", GroupID: 20}},
		{name: "configured event", groupID: 2, want: Role{Name: "Event", GroupID: 2}},
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

func TestRoleResolver_GetRole(t *testing.T) {
	t.Parallel()

	resolver := NewRoleResolver(config.RolesConfig{
		"Moderator": 20,
		"Enforcer":  10,
	})

	tests := []struct {
		name   string
		query  string
		want   Role
		wantOk bool
	}{
		{name: "authenticated sentinel", query: "*", want: RoleAuthenticated, wantOk: true},
		{name: "public sentinel", query: "Public", want: RolePublic, wantOk: true},
		{name: "admin hardcoded", query: "Admin", want: RoleAdmin, wantOk: true},
		{name: "configured moderator", query: "Moderator", want: Role{Name: "Moderator", GroupID: 20}, wantOk: true},
		{name: "configured enforcer", query: "Enforcer", want: Role{Name: "Enforcer", GroupID: 10}, wantOk: true},
		{name: "unknown role", query: "VIP", want: Role{}, wantOk: false},
		{name: "lowercase mismatch", query: "moderator", want: Role{}, wantOk: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := resolver.GetRole(tt.query)
			if ok != tt.wantOk {
				t.Errorf("GetRole(%q) ok = %v, want %v", tt.query, ok, tt.wantOk)
			}
			if got != tt.want {
				t.Errorf("GetRole(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

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

func TestUser_IsPlayer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		groupID int
		want    bool
	}{
		{"player", 0, true},
		{"moderator", 20, false},
		{"admin", 99, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			u := &User{GroupID: tt.groupID}
			if got := u.IsPlayer(); got != tt.want {
				t.Errorf("IsPlayer() = %v, want %v", got, tt.want)
			}
		})
	}
}
