package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeRolesYAML(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "roles.yml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func TestParseRolesConfig(t *testing.T) {
	t.Parallel()

	body := `
News:
  View:
  Create: ["Moderator"]
  Edit:   ["Moderator", "Enforcer"]

Events:
  View:   ["*"]
  Manage: ["Event", "Moderator"]
`
	cfg, err := parseRolesConfig([]byte(body))
	if err != nil {
		t.Fatalf("parseRolesConfig: %v", err)
	}

	want := RolesConfig{
		"News": ActionRoles{
			"View":   nil,
			"Create": RoleList{"Moderator"},
			"Edit":   RoleList{"Moderator", "Enforcer"},
		},
		"Events": ActionRoles{
			"View":   RoleList{"*"},
			"Manage": RoleList{"Event", "Moderator"},
		},
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("parsed = %#v\nwant   = %#v", cfg, want)
	}
}

func TestParseRolesConfig_Empty(t *testing.T) {
	t.Parallel()
	cfg, err := parseRolesConfig(nil)
	if err != nil {
		t.Fatalf("parseRolesConfig: %v", err)
	}
	if len(cfg) != 0 {
		t.Errorf("empty file produced %#v, want empty", cfg)
	}
}

func mustPanicMessage(t *testing.T, fn func()) string {
	t.Helper()
	var got any
	func() {
		defer func() { got = recover() }()
		fn()
	}()
	if got == nil {
		t.Fatalf("expected panic, got none")
	}
	if e, ok := got.(error); ok {
		return e.Error()
	}
	if s, ok := got.(string); ok {
		return s
	}
	t.Fatalf("unexpected panic value %T: %v", got, got)
	return ""
}

func TestValidateRolesConfig_RejectsInvalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		cfg         RolesConfig
		name        string
		wantContain string
	}{
		{
			name:        "admin top-level key",
			wantContain: "Admin is hardcoded",
			cfg:         RolesConfig{"Admin": ActionRoles{"Dashboard": nil}},
		},
		{
			name:        "empty action list",
			wantContain: "Action 'News.Edit' has an empty list",
			cfg:         RolesConfig{"News": ActionRoles{"Edit": RoleList{}}},
		},
		{
			name:        "unknown role",
			wantContain: "unknown role 'Modartor'",
			cfg:         RolesConfig{"News": ActionRoles{"Edit": RoleList{"Modartor"}}},
		},
		{
			name:        "admin inside list",
			wantContain: "Admin is implicit",
			cfg:         RolesConfig{"News": ActionRoles{"Edit": RoleList{"Admin"}}},
		},
		{
			name:        "lowercase role typo",
			wantContain: "unknown role 'moderator'",
			cfg:         RolesConfig{"News": ActionRoles{"Edit": RoleList{"moderator"}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := tt.cfg
			msg := mustPanicMessage(t, func() { validateRolesConfig(cfg) })
			if !strings.Contains(msg, tt.wantContain) {
				t.Errorf("panic message = %q, want substring %q", msg, tt.wantContain)
			}
		})
	}
}

func TestValidateRolesConfig_AcceptsHappyPath(t *testing.T) {
	t.Parallel()
	cfg := RolesConfig{
		"News": ActionRoles{
			"View":   nil,
			"Create": RoleList{"Moderator"},
			"Edit":   RoleList{"*"},
		},
	}
	validateRolesConfig(cfg)
}

func TestProcessRolesConfig_ReadsFile(t *testing.T) {
	t.Parallel()
	path := writeRolesYAML(t, "News:\n  Edit: [\"Moderator\"]\n")

	cfg, err := loadRolesConfig(path)
	if err != nil {
		t.Fatalf("loadRolesConfig: %v", err)
	}
	if got := cfg["News"]["Edit"]; !reflect.DeepEqual(got, RoleList{"Moderator"}) {
		t.Errorf("News.Edit = %#v, want [\"Moderator\"]", got)
	}
}
