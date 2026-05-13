package config

import (
	"strings"
	"testing"
)

func mustPanic(t *testing.T, fn func()) string {
	t.Helper()
	var got any
	func() {
		defer func() { got = recover() }()
		fn()
	}()
	if got == nil {
		t.Fatalf("expected panic, got none")
	}
	switch value := got.(type) {
	case error:
		return value.Error()
	case string:
		return value
	default:
		t.Fatalf("unexpected panic type %T: %v", got, got)
		return ""
	}
}

func TestValidateRolesConfig_AcceptsDistinctPositiveNonReservedValues(t *testing.T) {
	t.Parallel()
	cfg := RolesConfig{"Moderator": 20, "Enforcer": 10, "Event": 2, "VIP": 5}
	validateRolesConfig(cfg)
}

func TestValidateRolesConfig_AcceptsEmptyMap(t *testing.T) {
	t.Parallel()
	validateRolesConfig(RolesConfig{})
}

func TestValidateRolesConfig_RejectsInvalidValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		cfg         RolesConfig
		name        string
		wantContain string
	}{
		{
			name:        "zero value",
			cfg:         RolesConfig{"Moderator": 0},
			wantContain: "UserRoles.Moderator must be > 0",
		},
		{
			name:        "negative value",
			cfg:         RolesConfig{"Enforcer": -1},
			wantContain: "UserRoles.Enforcer must be > 0",
		},
		{
			name:        "reserved 99 for non-admin role",
			cfg:         RolesConfig{"Moderator": 99},
			wantContain: "UserRoles.Moderator = 99 is reserved for admin",
		},
		{
			name:        "reserved name Admin",
			cfg:         RolesConfig{"Admin": 50},
			wantContain: "UserRoles.Admin is reserved",
		},
		{
			name:        "reserved name star",
			cfg:         RolesConfig{"*": 50},
			wantContain: "UserRoles.* is reserved",
		},
		{
			name:        "reserved name Player",
			cfg:         RolesConfig{"Player": 50},
			wantContain: "UserRoles.Player is reserved",
		},
		{
			name:        "duplicate group_id",
			cfg:         RolesConfig{"Moderator": 10, "Enforcer": 10},
			wantContain: "shares group_id 10",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := tt.cfg
			msg := mustPanic(t, func() { validateRolesConfig(cfg) })
			if !strings.Contains(msg, tt.wantContain) {
				t.Errorf("panic message = %q, want substring %q", msg, tt.wantContain)
			}
		})
	}
}
