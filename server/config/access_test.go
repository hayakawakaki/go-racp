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
			name:        "admin alongside other roles",
			wantContain: "Admin is implicit",
			cfg:         AccessConfig{"News": ActionRoles{"Edit": Entry{Roles: RoleList{"Admin", "Moderator"}}}},
		},
		{
			name:        "sole admin combined with unrestricted",
			wantContain: "Admin",
			cfg: AccessConfig{
				"Users": ActionRoles{
					"List": Entry{Roles: RoleList{"Admin"}, Requires: []string{"Unrestricted"}},
				},
			},
		},
		{
			name:        "empty role name",
			wantContain: "empty role name",
			cfg:         AccessConfig{"News": ActionRoles{"Edit": Entry{Roles: RoleList{""}}}},
		},
		{
			name:        "public mixed with other role",
			wantContain: "Public",
			cfg:         AccessConfig{"News": ActionRoles{"Edit": Entry{Roles: RoleList{"Public", "Moderator"}}}},
		},
		{
			name:        "public combined with unrestricted",
			wantContain: "Public",
			cfg:         AccessConfig{"News": ActionRoles{"View": Entry{Roles: RoleList{"Public"}, Requires: []string{"Unrestricted"}}}},
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
		"Home": ActionRoles{
			"View": Entry{Roles: RoleList{"Public"}},
		},
		"Users": ActionRoles{
			"List": Entry{Roles: RoleList{"Admin"}},
			"Ban":  Entry{Roles: RoleList{"Moderator", "Enforcer"}, Requires: []string{"Unrestricted"}},
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

func TestMergeAccessConfigs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		root  AccessConfig
		theme AccessConfig
		want  AccessConfig
		name  string
	}{
		{
			name:  "both empty",
			root:  AccessConfig{},
			theme: AccessConfig{},
			want:  AccessConfig{},
		},
		{
			name: "non-empty root with empty theme",
			root: AccessConfig{
				"News": ActionRoles{"View": Entry{Roles: RoleList{"Public"}}},
			},
			theme: AccessConfig{},
			want: AccessConfig{
				"News": ActionRoles{"View": Entry{Roles: RoleList{"Public"}}},
			},
		},
		{
			name: "empty root with non-empty theme",
			root: AccessConfig{},
			theme: AccessConfig{
				"ThemePages": ActionRoles{"Rates": Entry{Roles: RoleList{"Public"}}},
			},
			want: AccessConfig{
				"ThemePages": ActionRoles{"Rates": Entry{Roles: RoleList{"Public"}}},
			},
		},
		{
			name: "disjoint groups merge",
			root: AccessConfig{
				"News": ActionRoles{"View": Entry{Roles: RoleList{"Public"}}},
			},
			theme: AccessConfig{
				"ThemePages": ActionRoles{"Rates": Entry{Roles: RoleList{"Public"}}},
			},
			want: AccessConfig{
				"News":       ActionRoles{"View": Entry{Roles: RoleList{"Public"}}},
				"ThemePages": ActionRoles{"Rates": Entry{Roles: RoleList{"Public"}}},
			},
		},
		{
			name: "same group different actions merge",
			root: AccessConfig{
				"ThemePages": ActionRoles{"Rates": Entry{Roles: RoleList{"Public"}}},
			},
			theme: AccessConfig{
				"ThemePages": ActionRoles{"ServerInfo": Entry{Roles: RoleList{"Public"}}},
			},
			want: AccessConfig{
				"ThemePages": ActionRoles{
					"Rates":      Entry{Roles: RoleList{"Public"}},
					"ServerInfo": Entry{Roles: RoleList{"Public"}},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := mergeAccessConfigs(tt.root, tt.theme)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("merge = %#v\nwant  = %#v", got, tt.want)
			}
		})
	}
}

func TestMergeAccessConfigs_CollisionPanics(t *testing.T) {
	t.Parallel()

	root := AccessConfig{
		"ThemePages": ActionRoles{"Rates": Entry{Roles: RoleList{"Public"}}},
	}
	theme := AccessConfig{
		"ThemePages": ActionRoles{"Rates": Entry{Roles: RoleList{"Admin"}}},
	}

	msg := mustPanicMessage(t, func() { mergeAccessConfigs(root, theme) })
	if !strings.Contains(msg, "ThemePages.Rates") {
		t.Errorf("panic message = %q, want substring %q", msg, "ThemePages.Rates")
	}
	if !strings.Contains(msg, "defined in both") {
		t.Errorf("panic message = %q, want substring %q", msg, "defined in both")
	}
}

func TestMergeAccessConfigs_RootMutationAfterMergeDoesNotAffectResult(t *testing.T) {
	t.Parallel()

	root := AccessConfig{
		"ThemePages": ActionRoles{"Rates": Entry{Roles: RoleList{"Public"}}},
	}
	theme := AccessConfig{}

	merged := mergeAccessConfigs(root, theme)

	root["ThemePages"]["Rates"] = Entry{Roles: RoleList{"Admin"}}
	root["ThemePages"]["Mutated"] = Entry{Roles: RoleList{"Public"}}

	got := merged["ThemePages"]["Rates"]
	if !reflect.DeepEqual(got, Entry{Roles: RoleList{"Public"}}) {
		t.Errorf("merged entry mutated by root edit: got %#v", got)
	}
	if _, has := merged["ThemePages"]["Mutated"]; has {
		t.Errorf("merged group mutated by root add: got Mutated key in merged result")
	}
}

func TestValidateThemeAccessConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		cfg         AccessConfig
		name        string
		wantContain string
	}{
		{
			name:        "empty config accepted",
			cfg:         AccessConfig{},
			wantContain: "",
		},
		{
			name: "themepages only accepted",
			cfg: AccessConfig{
				"ThemePages": ActionRoles{
					"Rates":      Entry{Roles: RoleList{"Public"}},
					"ServerInfo": Entry{Roles: RoleList{"Public"}},
				},
			},
			wantContain: "",
		},
		{
			name: "slice group rejected",
			cfg: AccessConfig{
				"News": ActionRoles{"View": Entry{Roles: RoleList{"Public"}}},
			},
			wantContain: `"News"`,
		},
		{
			name: "admin group rejected",
			cfg: AccessConfig{
				"Admin": ActionRoles{"Dashboard": Entry{}},
			},
			wantContain: `"Admin"`,
		},
		{
			name: "themepages plus rogue group rejected",
			cfg: AccessConfig{
				"ThemePages": ActionRoles{"Rates": Entry{Roles: RoleList{"Public"}}},
				"News":       ActionRoles{"View": Entry{Roles: RoleList{"Public"}}},
			},
			wantContain: `"News"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := tt.cfg

			if tt.wantContain == "" {
				validateThemeAccessConfig(cfg)

				return
			}

			msg := mustPanicMessage(t, func() { validateThemeAccessConfig(cfg) })
			if !strings.Contains(msg, tt.wantContain) {
				t.Errorf("panic message = %q, want substring %q", msg, tt.wantContain)
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
