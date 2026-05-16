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
			"View":   Entry{},
			"Create": Entry{Roles: RoleList{"Moderator"}},
			"Edit":   Entry{Roles: RoleList{"Moderator", "Enforcer"}},
		},
		"Events": ActionRoles{
			"View":   Entry{Roles: RoleList{"*"}},
			"Manage": Entry{Roles: RoleList{"Event", "Moderator"}},
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
			cfg:         AccessConfig{"Admin": ActionRoles{"Dashboard": Entry{}}},
		},
		{
			name:        "empty action list",
			wantContain: "Action 'News.Edit' has an empty roles list",
			cfg:         AccessConfig{"News": ActionRoles{"Edit": Entry{Roles: RoleList{}}}},
		},
		{
			name:        "admin inside list",
			wantContain: "Admin is implicit",
			cfg:         AccessConfig{"News": ActionRoles{"Edit": Entry{Roles: RoleList{"Admin"}}}},
		},
		{
			name:        "empty role name",
			wantContain: "empty role name",
			cfg:         AccessConfig{"News": ActionRoles{"Edit": Entry{Roles: RoleList{""}}}},
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
			"View":   Entry{},
			"Create": Entry{Roles: RoleList{"Moderator"}},
			"Edit":   Entry{Roles: RoleList{"*"}},
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
	if got := cfg["News"]["Edit"]; !reflect.DeepEqual(got, Entry{Roles: RoleList{"Moderator"}}) {
		t.Errorf("News.Edit = %#v, want [\"Moderator\"]", got)
	}
}

func TestParseAccessConfig_StructFormDecodesIntoEntry(t *testing.T) {
	t.Parallel()

	body := `
Account:
  View: ["*"]
  ChangeEmail:
    Roles: ["*"]
    Requires: ["Unrestricted"]
  ChangePassword:
    Roles: ["Moderator", "Enforcer"]
    Requires: ["Unrestricted"]
`
	cfg, err := parseAccessConfig([]byte(body))
	if err != nil {
		t.Fatalf("parseAccessConfig: %v", err)
	}

	want := AccessConfig{
		"Account": ActionRoles{
			"View":           Entry{Roles: RoleList{"*"}},
			"ChangeEmail":    Entry{Roles: RoleList{"*"}, Requires: []string{"Unrestricted"}},
			"ChangePassword": Entry{Roles: RoleList{"Moderator", "Enforcer"}, Requires: []string{"Unrestricted"}},
		},
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("parsed = %#v\nwant   = %#v", cfg, want)
	}
}

func TestParseAccessConfig_ShorthandAndStructProduceSameEntry(t *testing.T) {
	t.Parallel()

	shorthandBody := []byte(`Account:
  View: ["*"]
`)
	structBody := []byte(`Account:
  View:
    Roles: ["*"]
`)
	shorthand, err := parseAccessConfig(shorthandBody)
	if err != nil {
		t.Fatalf("shorthand parse: %v", err)
	}
	structForm, err := parseAccessConfig(structBody)
	if err != nil {
		t.Fatalf("struct parse: %v", err)
	}
	if !reflect.DeepEqual(shorthand, structForm) {
		t.Errorf("shorthand and struct must yield equal Entry values\nshorthand = %#v\nstruct    = %#v", shorthand, structForm)
	}
}

func TestValidateAccessConfig_UnknownRequiresTagPanics(t *testing.T) {
	t.Parallel()

	cfg := AccessConfig{
		"Account": ActionRoles{
			"ChangePassword": Entry{Roles: RoleList{"*"}, Requires: []string{"Mystery"}},
		},
	}
	msg := mustPanicMessage(t, func() { validateAccessConfig(cfg) })
	if !strings.Contains(msg, "unknown requires tag") {
		t.Errorf("panic message = %q, want substring %q", msg, "unknown requires tag")
	}
	if !strings.Contains(msg, "Mystery") {
		t.Errorf("panic message = %q, want to mention bad tag Mystery", msg)
	}
	if !strings.Contains(msg, "Account.ChangePassword") {
		t.Errorf("panic message = %q, want to mention Account.ChangePassword", msg)
	}
}

func TestValidateAccessConfig_UnknownRequiresTagPanicsEvenWhenRolesNil(t *testing.T) {
	t.Parallel()

	cfg := AccessConfig{
		"Account": ActionRoles{
			"ChangePassword": Entry{Requires: []string{"BogusTag"}},
		},
	}
	msg := mustPanicMessage(t, func() { validateAccessConfig(cfg) })
	if !strings.Contains(msg, "BogusTag") {
		t.Errorf("nil-Roles regression: panic must surface the unknown tag, got %q", msg)
	}
}

func TestValidateAccessConfig_KnownRequiresTagAccepted(t *testing.T) {
	t.Parallel()

	cfg := AccessConfig{
		"Account": ActionRoles{
			"ChangePassword": Entry{Roles: RoleList{"*"}, Requires: []string{RequireUnrestricted}},
		},
	}
	validateAccessConfig(cfg)
}

func TestEntry_RequiresUnrestricted(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		entry Entry
		want  bool
	}{
		{name: "no requires", entry: Entry{Roles: RoleList{"*"}}, want: false},
		{name: "empty requires", entry: Entry{Roles: RoleList{"*"}, Requires: []string{}}, want: false},
		{name: "unrestricted tag", entry: Entry{Roles: RoleList{"*"}, Requires: []string{"Unrestricted"}}, want: true},
		{name: "unrestricted among others", entry: Entry{Roles: RoleList{"*"}, Requires: []string{"Other", "Unrestricted"}}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.entry.RequiresUnrestricted(); got != tt.want {
				t.Errorf("RequiresUnrestricted() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAccessConfig_ManageRoles(t *testing.T) {
	t.Parallel()

	cfg := AccessConfig{
		"Tickets": ActionRoles{"Manage": Entry{Roles: RoleList{"Moderator", "Enforcer"}}},
		"News":    ActionRoles{"Manage": Entry{Roles: RoleList{"Moderator"}}},
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
