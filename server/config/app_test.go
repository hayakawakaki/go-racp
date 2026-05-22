package config

import (
	"strings"
	"testing"
	"time"
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
			name:        "reserved name Public",
			cfg:         RolesConfig{"Public": 50},
			wantContain: "UserRoles.Public is reserved",
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

func TestValidateVendorConfig_Clamps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   time.Duration
		want time.Duration
	}{
		{name: "zero defaults to 30s", in: 0, want: 30 * time.Second},
		{name: "negative defaults to 30s", in: -1 * time.Second, want: 30 * time.Second},
		{name: "below min clamps to 5s", in: 1 * time.Second, want: 5 * time.Second},
		{name: "exactly min stays", in: 5 * time.Second, want: 5 * time.Second},
		{name: "in range stays", in: 45 * time.Second, want: 45 * time.Second},
		{name: "exactly max stays", in: 10 * time.Minute, want: 10 * time.Minute},
		{name: "above max clamps to 10m", in: 1 * time.Hour, want: 10 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := VendorConfig{PollInterval: tt.in}
			validateVendorConfig(&cfg)
			if cfg.PollInterval != tt.want {
				t.Errorf("PollInterval = %v, want %v", cfg.PollInterval, tt.want)
			}
		})
	}
}

func TestValidateTheme_AcceptsValidNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "default theme", in: "default", want: "default"},
		{name: "empty falls back to default", in: "", want: "default"},
		{name: "lowercase letters", in: "midnight", want: "midnight"},
		{name: "lowercase with digits", in: "theme01", want: "theme01"},
		{name: "lowercase with underscore", in: "my_theme", want: "my_theme"},
		{name: "all digits", in: "42", want: "42"},
		{name: "single letter", in: "x", want: "x"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := GeneralConfig{Theme: tt.in}
			validateTheme(&cfg)
			if cfg.Theme != tt.want {
				t.Errorf("Theme = %q, want %q", cfg.Theme, tt.want)
			}
		})
	}
}

func TestValidateTheme_RejectsInvalidNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		in          string
		wantContain string
	}{
		{name: "uppercase letter", in: "Midnight", wantContain: "must match"},
		{name: "all uppercase", in: "DEFAULT", wantContain: "must match"},
		{name: "space inside", in: "my theme", wantContain: "must match"},
		{name: "leading space", in: " midnight", wantContain: "must match"},
		{name: "trailing space", in: "midnight ", wantContain: "must match"},
		{name: "hyphen rejected", in: "my-theme", wantContain: "must match"},
		{name: "dot rejected", in: "my.theme", wantContain: "must match"},
		{name: "slash rejected", in: "my/theme", wantContain: "must match"},
		{name: "exclamation rejected", in: "midnight!", wantContain: "must match"},
		{name: "shell injection semicolon", in: "default; rm -rf /", wantContain: "must match"},
		{name: "shell injection backtick", in: "default`whoami`", wantContain: "must match"},
		{name: "newline rejected", in: "default\n", wantContain: "must match"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := GeneralConfig{Theme: tt.in}
			msg := mustPanic(t, func() { validateTheme(&cfg) })
			if !strings.Contains(msg, tt.wantContain) {
				t.Errorf("panic message = %q, want substring %q", msg, tt.wantContain)
			}
		})
	}
}
