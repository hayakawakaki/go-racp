package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestQualifyTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		params     string
		localAlias string
		want       string
	}{
		{
			name:       "empty params",
			params:     "",
			localAlias: "home",
			want:       "",
		},
		{
			name:       "single unqualified caps type",
			params:     "state HomeState",
			localAlias: "home",
			want:       "state home.HomeState",
		},
		{
			name:       "already qualified type is left alone",
			params:     "layout httpx.Layout",
			localAlias: "home",
			want:       "layout httpx.Layout",
		},
		{
			name:       "builtin types are not qualified",
			params:     "title string, count int, ok bool",
			localAlias: "home",
			want:       "title string, count int, ok bool",
		},
		{
			name:       "slice prefix preserved",
			params:     "items []HomeState",
			localAlias: "home",
			want:       "items []home.HomeState",
		},
		{
			name:       "pointer prefix preserved",
			params:     "state *HomeState",
			localAlias: "home",
			want:       "state *home.HomeState",
		},
		{
			name:       "map value type qualified",
			params:     "lookup map[string]HomeState",
			localAlias: "home",
			want:       "lookup map[string]home.HomeState",
		},
		{
			name:       "mix of qualified, unqualified, and builtins",
			params:     "layout httpx.Layout, state HomeState, count int",
			localAlias: "home",
			want:       "layout httpx.Layout, state home.HomeState, count int",
		},
		{
			name:       "templ.Component already qualified",
			params:     "content templ.Component",
			localAlias: "admin",
			want:       "content templ.Component",
		},
		{
			name:       "domain reference already qualified",
			params:     "messages []domain.Message, isStaff bool",
			localAlias: "tickets",
			want:       "messages []domain.Message, isStaff bool",
		},
		{
			name:       "multiple unqualified types all get prefix",
			params:     "a Foo, b Bar, c Baz",
			localAlias: "x",
			want:       "a x.Foo, b x.Bar, c x.Baz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := qualifyTypes(tt.params, tt.localAlias)
			if got != tt.want {
				t.Errorf("qualifyTypes(%q, %q) = %q, want %q", tt.params, tt.localAlias, got, tt.want)
			}
		})
	}
}

func TestAliasFor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		dir  string
		want string
	}{
		{
			name: "slice transport",
			dir:  "internal/features/home/transport",
			want: "home",
		},
		{
			name: "platform httpx",
			dir:  "internal/platform/httpx",
			want: "httpx",
		},
		{
			name: "nested transport subdir",
			dir:  "internal/features/account/transport/self",
			want: "accountself",
		},
		{
			name: "moderation subdir",
			dir:  "internal/features/account/transport/moderation",
			want: "accountmoderation",
		},
		{
			name: "notifications subdir",
			dir:  "internal/features/tickets/notifications",
			want: "ticketsnotifications",
		},
		{
			name: "tickets transport",
			dir:  "internal/features/tickets/transport",
			want: "tickets",
		},
		{
			name: "only skip segments falls back to all",
			dir:  "internal/platform",
			want: "internalplatform",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := aliasFor(tt.dir)
			if got != tt.want {
				t.Errorf("aliasFor(%q) = %q, want %q", tt.dir, got, tt.want)
			}
		})
	}
}

func TestParamNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		params string
		want   string
	}{
		{
			name:   "empty",
			params: "",
			want:   "",
		},
		{
			name:   "single param",
			params: "state HomeState",
			want:   "state",
		},
		{
			name:   "two params",
			params: "layout httpx.Layout, state HomeState",
			want:   "layout, state",
		},
		{
			name:   "grouped type declaration",
			params: "a, b int",
			want:   "a, b",
		},
		{
			name:   "multi-arg ticket signature",
			params: "messages []domain.Message, isStaff bool, otherSeenAt time.Time",
			want:   "messages, isStaff, otherSeenAt",
		},
		{
			name:   "only whitespace yields empty",
			params: "   ",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := paramNames(tt.params)
			if got != tt.want {
				t.Errorf("paramNames(%q) = %q, want %q", tt.params, got, tt.want)
			}
		})
	}
}

func TestParseTemplImports(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  string
		want []importEntry
	}{
		{
			name: "no import block",
			src:  "package transport\n\ntempl Foo() {}\n",
			want: nil,
		},
		{
			name: "single unaliased import",
			src:  "package transport\n\nimport (\n\t\"time\"\n)\n",
			want: []importEntry{{Alias: "time", Path: "time"}},
		},
		{
			name: "single aliased import",
			src:  "package transport\n\nimport (\n\tdomain \"github.com/x/y/domain\"\n)\n",
			want: []importEntry{{Alias: "domain", Path: "github.com/x/y/domain"}},
		},
		{
			name: "multiple mixed imports",
			src: "package transport\n\nimport (\n" +
				"\t\"time\"\n" +
				"\tdomain \"github.com/x/tickets/domain\"\n" +
				"\t\"github.com/x/internal/platform/httpx\"\n" +
				")\n",
			want: []importEntry{
				{Alias: "time", Path: "time"},
				{Alias: "domain", Path: "github.com/x/tickets/domain"},
				{Alias: "httpx", Path: "github.com/x/internal/platform/httpx"},
			},
		},
		{
			name: "unaliased import uses last path segment",
			src:  "package transport\n\nimport (\n\t\"github.com/foo/bar/baz\"\n)\n",
			want: []importEntry{{Alias: "baz", Path: "github.com/foo/bar/baz"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseTemplImports(tt.src)
			if !equalImports(got, tt.want) {
				t.Errorf("parseTemplImports() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReferencedImports(t *testing.T) {
	t.Parallel()

	fileImports := []importEntry{
		{Alias: "domain", Path: "github.com/x/tickets/domain"},
		{Alias: "time", Path: "time"},
		{Alias: "httpx", Path: "github.com/x/platform/httpx"},
	}

	tests := []struct {
		name        string
		params      string
		fileImports []importEntry
		want        []importEntry
	}{
		{
			name:        "no qualified types returns nil",
			params:      "state HomeState, count int",
			fileImports: fileImports,
			want:        nil,
		},
		{
			name:        "empty file imports returns nil",
			params:      "x domain.Ticket",
			fileImports: nil,
			want:        nil,
		},
		{
			name:        "single qualified type matching imports",
			params:      "x domain.Ticket",
			fileImports: fileImports,
			want:        []importEntry{{Alias: "domain", Path: "github.com/x/tickets/domain"}},
		},
		{
			name:        "qualified type not in file imports is skipped",
			params:      "x other.Foo",
			fileImports: fileImports,
			want:        nil,
		},
		{
			name:        "multiple qualified types all returned",
			params:      "messages []domain.Message, otherSeenAt time.Time",
			fileImports: fileImports,
			want: []importEntry{
				{Alias: "domain", Path: "github.com/x/tickets/domain"},
				{Alias: "time", Path: "time"},
			},
		},
		{
			name:        "duplicate alias references deduplicated",
			params:      "a domain.Ticket, b domain.Message",
			fileImports: fileImports,
			want:        []importEntry{{Alias: "domain", Path: "github.com/x/tickets/domain"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := referencedImports(tt.params, tt.fileImports)
			if !equalImports(got, tt.want) {
				t.Errorf("referencedImports(%q) = %v, want %v", tt.params, got, tt.want)
			}
		})
	}
}

func TestUniqueImports(t *testing.T) {
	t.Parallel()

	t.Run("empty components", func(t *testing.T) {
		t.Parallel()
		got, err := uniqueImports(nil)
		if err != nil {
			t.Fatalf("uniqueImports() error = %v", err)
		}
		if len(got) != 0 {
			t.Errorf("uniqueImports() returned %d entries, want 0", len(got))
		}
	})

	t.Run("single component single import", func(t *testing.T) {
		t.Parallel()
		comps := []Component{{
			ImportName: "home",
			ImportPath: "github.com/x/home/transport",
		}}
		got, err := uniqueImports(comps)
		if err != nil {
			t.Fatalf("uniqueImports() error = %v", err)
		}
		if !containsImport(got, importEntry{Alias: "home", Path: "github.com/x/home/transport"}) {
			t.Errorf("missing expected entry, got %v", got)
		}
	})

	t.Run("merges extra imports", func(t *testing.T) {
		t.Parallel()
		comps := []Component{{
			ImportName: "tickets",
			ImportPath: "github.com/x/tickets/transport",
			ExtraImports: []importEntry{
				{Alias: "domain", Path: "github.com/x/tickets/domain"},
				{Alias: "time", Path: "time"},
			},
		}}
		got, err := uniqueImports(comps)
		if err != nil {
			t.Fatalf("uniqueImports() error = %v", err)
		}
		if !containsImport(got, importEntry{Alias: "tickets", Path: "github.com/x/tickets/transport"}) ||
			!containsImport(got, importEntry{Alias: "domain", Path: "github.com/x/tickets/domain"}) ||
			!containsImport(got, importEntry{Alias: "time", Path: "time"}) {
			t.Errorf("missing expected entries, got %v", got)
		}
	})

	t.Run("dedups across components", func(t *testing.T) {
		t.Parallel()
		comps := []Component{
			{ImportName: "home", ImportPath: "github.com/x/home/transport"},
			{ImportName: "home", ImportPath: "github.com/x/home/transport"},
		}
		got, err := uniqueImports(comps)
		if err != nil {
			t.Fatalf("uniqueImports() error = %v", err)
		}
		if len(got) != 1 {
			t.Errorf("expected 1 entry after dedup, got %d: %v", len(got), got)
		}
	})

	t.Run("alias collision on ImportName returns error", func(t *testing.T) {
		t.Parallel()
		comps := []Component{
			{ImportName: "domain", ImportPath: "github.com/x/tickets/domain"},
			{ImportName: "domain", ImportPath: "github.com/x/news/domain"},
		}
		_, err := uniqueImports(comps)
		if err == nil {
			t.Fatal("expected collision error, got nil")
		}
		if !strings.Contains(err.Error(), "alias collision") {
			t.Errorf("error = %v, want substring %q", err, "alias collision")
		}
	})

	t.Run("alias collision on ExtraImports returns error", func(t *testing.T) {
		t.Parallel()
		comps := []Component{
			{
				ImportName: "tickets",
				ImportPath: "github.com/x/tickets/transport",
				ExtraImports: []importEntry{
					{Alias: "domain", Path: "github.com/x/tickets/domain"},
				},
			},
			{
				ImportName: "news",
				ImportPath: "github.com/x/news/transport",
				ExtraImports: []importEntry{
					{Alias: "domain", Path: "github.com/x/news/domain"},
				},
			},
		}
		_, err := uniqueImports(comps)
		if err == nil {
			t.Fatal("expected collision error, got nil")
		}
		if !strings.Contains(err.Error(), "alias collision") {
			t.Errorf("error = %v, want substring %q", err, "alias collision")
		}
	})
}

func TestFormatImport(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		imp  importEntry
		want string
	}{
		{
			name: "alias matches last segment drops alias",
			imp:  importEntry{Alias: "time", Path: "time"},
			want: "\t\"time\"\n",
		},
		{
			name: "alias matches multi-segment last drops alias",
			imp:  importEntry{Alias: "domain", Path: "github.com/x/tickets/domain"},
			want: "\t\"github.com/x/tickets/domain\"\n",
		},
		{
			name: "alias differs from last segment kept",
			imp:  importEntry{Alias: "home", Path: "github.com/x/home/transport"},
			want: "\thome \"github.com/x/home/transport\"\n",
		},
		{
			name: "explicit collision alias kept",
			imp:  importEntry{Alias: "mobapp", Path: "github.com/x/mob/app"},
			want: "\tmobapp \"github.com/x/mob/app\"\n",
		},
		{
			name: "httpx alias matches last segment",
			imp:  importEntry{Alias: "httpx", Path: "github.com/x/platform/httpx"},
			want: "\t\"github.com/x/platform/httpx\"\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := formatImport(tt.imp)
			if got != tt.want {
				t.Errorf("formatImport(%+v) = %q, want %q", tt.imp, got, tt.want)
			}
		})
	}
}

func TestWriteIfChanged_NewFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")

	if err := writeIfChanged(path, []byte("hello")); err != nil {
		t.Fatalf("writeIfChanged() error = %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("content = %q, want %q", got, "hello")
	}
}

func TestWriteIfChanged_DifferentContent_Writes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")

	if err := os.WriteFile(path, []byte("hello"), filePerm); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := writeIfChanged(path, []byte("world")); err != nil {
		t.Fatalf("writeIfChanged() error = %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != "world" {
		t.Errorf("content = %q, want %q", got, "world")
	}
}

func TestWriteIfChanged_SameContent_DoesNotTouchMtime(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")

	if err := os.WriteFile(path, []byte("hello"), filePerm); err != nil {
		t.Fatalf("setup: %v", err)
	}

	oldTime := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, oldTime, oldTime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	if err := writeIfChanged(path, []byte("hello")); err != nil {
		t.Fatalf("writeIfChanged() error = %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if !info.ModTime().Equal(oldTime) {
		t.Errorf("mtime changed from %v to %v on no-op write", oldTime, info.ModTime())
	}
}

func equalImports(a, b []importEntry) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func containsImport(haystack []importEntry, needle importEntry) bool {
	return slices.Contains(haystack, needle)
}
