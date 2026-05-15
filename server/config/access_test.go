package config

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func writeAccessYAML(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "access.yml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func TestParseAccessConfig(t *testing.T) {
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
	cfg, err := parseAccessConfig([]byte(body))
	if err != nil {
		t.Fatalf("parseAccessConfig: %v", err)
	}

	want := AccessConfig{
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

func TestParseAccessConfig_Empty(t *testing.T) {
	t.Parallel()
	cfg, err := parseAccessConfig(nil)
	if err != nil {
		t.Fatalf("parseAccessConfig: %v", err)
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

func TestValidateAccessConfig_RejectsInvalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		cfg         AccessConfig
		name        string
		wantContain string
	}{
		{
			name:        "admin top-level key",
			wantContain: "Admin is hardcoded",
			cfg:         AccessConfig{"Admin": ActionRoles{"Dashboard": nil}},
		},
		{
			name:        "empty action list",
			wantContain: "Action 'News.Edit' has an empty list",
			cfg:         AccessConfig{"News": ActionRoles{"Edit": RoleList{}}},
		},
		{
			name:        "admin inside list",
			wantContain: "Admin is implicit",
			cfg:         AccessConfig{"News": ActionRoles{"Edit": RoleList{"Admin"}}},
		},
		{
			name:        "empty role name",
			wantContain: "empty role name",
			cfg:         AccessConfig{"News": ActionRoles{"Edit": RoleList{""}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := tt.cfg
			msg := mustPanicMessage(t, func() { validateAccessConfig(cfg) })
			if !strings.Contains(msg, tt.wantContain) {
				t.Errorf("panic message = %q, want substring %q", msg, tt.wantContain)
			}
		})
	}
}

func TestValidateAccessConfig_AcceptsHappyPath(t *testing.T) {
	t.Parallel()
	cfg := AccessConfig{
		"News": ActionRoles{
			"View":   nil,
			"Create": RoleList{"Moderator"},
			"Edit":   RoleList{"*"},
		},
	}
	validateAccessConfig(cfg)
}

func TestLoadAccessConfig_ReadsFile(t *testing.T) {
	t.Parallel()
	path := writeAccessYAML(t, "News:\n  Edit: [\"Moderator\"]\n")

	cfg, err := loadAccessConfig(path)
	if err != nil {
		t.Fatalf("loadAccessConfig: %v", err)
	}
	if got := cfg["News"]["Edit"]; !reflect.DeepEqual(got, RoleList{"Moderator"}) {
		t.Errorf("News.Edit = %#v, want [\"Moderator\"]", got)
	}
}

func TestAccessConfig_ManageRoles(t *testing.T) {
	t.Parallel()

	cfg := AccessConfig{
		"Tickets": ActionRoles{"Manage": RoleList{"Moderator", "Enforcer"}},
		"News":    ActionRoles{"Manage": RoleList{"Moderator"}},
		"Empty":   ActionRoles{},
	}

	tests := []struct {
		name  string
		group string
		want  []string
	}{
		{"tickets returns its manage roles", "Tickets", []string{"Moderator", "Enforcer"}},
		{"news returns its manage roles", "News", []string{"Moderator"}},
		{"missing group returns nil", "Bogus", nil},
		{"group without Manage action returns nil", "Empty", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := cfg.ManageRoles(tt.group)
			if !slices.Equal(got, tt.want) {
				t.Errorf("ManageRoles(%q) = %v, want %v", tt.group, got, tt.want)
			}
		})
	}
}
