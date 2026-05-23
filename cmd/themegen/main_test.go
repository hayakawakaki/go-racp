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

func TestPascalCaseVarName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{name: "empty", input: "", expect: ""},
		{name: "single word", input: "rates", expect: "Rates"},
		{name: "dash separated", input: "server-info", expect: "ServerInfo"},
		{name: "underscore separated", input: "server_info", expect: "ServerInfo"},
		{name: "mixed dash underscore", input: "multi-word_name", expect: "MultiWordName"},
		{name: "slash separated nested path", input: "events/summer", expect: "EventsSummer"},
		{name: "slash plus dash", input: "events/summer-2026", expect: "EventsSummer2026"},
		{name: "trailing digit", input: "page2", expect: "Page2"},
		{name: "leading digit", input: "2foo", expect: "2foo"},
		{name: "only dashes", input: "--", expect: ""},
		{name: "only underscores", input: "__", expect: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := pascalCaseVarName(tt.input); got != tt.expect {
				t.Errorf("pascalCaseVarName(%q) = %q, want %q", tt.input, got, tt.expect)
			}
		})
	}
}

func TestThemePageTag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{name: "single word", input: "rates", expect: "ThemePages.Rates"},
		{name: "dash separated", input: "server-info", expect: "ThemePages.ServerInfo"},
		{name: "nested path", input: "events/summer-2026", expect: "ThemePages.EventsSummer2026"},
		{name: "empty input yields prefix only", input: "", expect: "ThemePages."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := themePageTag(tt.input); got != tt.expect {
				t.Errorf("themePageTag(%q) = %q, want %q", tt.input, got, tt.expect)
			}
		})
	}
}

func TestParseFrontmatter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		check     func(t *testing.T, fm frontmatter, body []byte)
		name      string
		input     string
		wantFound bool
	}{
		{
			name:      "no frontmatter",
			input:     "# Heading\nbody text\n",
			wantFound: false,
			check: func(t *testing.T, fm frontmatter, body []byte) {
				if fm.Title != "" {
					t.Errorf("Title = %q, want empty", fm.Title)
				}
				if string(body) != "# Heading\nbody text\n" {
					t.Errorf("body = %q, want original content", string(body))
				}
			},
		},
		{
			name:      "lf frontmatter with title",
			input:     "---\ntitle: Server Info\n---\n# Heading\n",
			wantFound: true,
			check: func(t *testing.T, fm frontmatter, body []byte) {
				if fm.Title != "Server Info" {
					t.Errorf("Title = %q, want %q", fm.Title, "Server Info")
				}
				if string(body) != "# Heading\n" {
					t.Errorf("body = %q, want %q", string(body), "# Heading\n")
				}
			},
		},
		{
			name:      "crlf frontmatter with title",
			input:     "---\r\ntitle: Server Info\r\n---\r\n# Heading\r\n",
			wantFound: true,
			check: func(t *testing.T, fm frontmatter, body []byte) {
				if fm.Title != "Server Info" {
					t.Errorf("Title = %q, want %q", fm.Title, "Server Info")
				}
				if !strings.Contains(string(body), "# Heading") {
					t.Errorf("body = %q, want to contain %q", string(body), "# Heading")
				}
			},
		},
		{
			name:      "frontmatter only no body",
			input:     "---\ntitle: Solo\n---\n",
			wantFound: true,
			check: func(t *testing.T, fm frontmatter, body []byte) {
				if fm.Title != "Solo" {
					t.Errorf("Title = %q, want %q", fm.Title, "Solo")
				}
				if len(body) != 0 {
					t.Errorf("body = %q, want empty", string(body))
				}
			},
		},
		{
			name:      "malformed yaml falls through as no frontmatter",
			input:     "---\ntitle: : : broken\n---\nbody\n",
			wantFound: false,
			check: func(t *testing.T, fm frontmatter, body []byte) {
				if fm.Title != "" {
					t.Errorf("Title = %q, want empty on malformed yaml", fm.Title)
				}
				if !strings.Contains(string(body), "title: : : broken") {
					t.Errorf("body should retain original content on malformed yaml")
				}
			},
		},
		{
			name:      "frontmatter without trailing newline does not match",
			input:     "---\ntitle: X\n---",
			wantFound: false,
			check: func(t *testing.T, _ frontmatter, body []byte) {
				if !strings.Contains(string(body), "title: X") {
					t.Errorf("body should retain original content when terminator newline missing")
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fm, body, found := parseFrontmatter([]byte(tt.input))
			if found != tt.wantFound {
				t.Errorf("found = %v, want %v", found, tt.wantFound)
			}
			tt.check(t, fm, body)
		})
	}
}

func TestExtractH1Title(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{name: "simple h1", input: "# Server Info\nbody\n", expect: "Server Info"},
		{name: "h1 with trailing whitespace", input: "# Server Info   \nbody\n", expect: "Server Info"},
		{name: "no h1", input: "body text only\n", expect: ""},
		{name: "h2 ignored", input: "## subheading\n", expect: ""},
		{name: "hash without space ignored", input: "#nospace\n", expect: ""},
		{name: "first of multiple h1s wins", input: "# First\nbody\n# Second\n", expect: "First"},
		{name: "h1 after body content", input: "intro paragraph\n\n# Buried Heading\n", expect: "Buried Heading"},
		{name: "empty input", input: "", expect: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := extractH1Title([]byte(tt.input)); got != tt.expect {
				t.Errorf("extractH1Title = %q, want %q", got, tt.expect)
			}
		})
	}
}

func TestCollectThemePages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		files       map[string]string
		check       func(t *testing.T, entries []pageEntry)
		name        string
		wantErrPart string
	}{
		{
			name: "single md page derives tag from filename",
			files: map[string]string{
				"rates.md": "# Server Rates\nbody\n",
			},
			check: func(t *testing.T, entries []pageEntry) {
				if len(entries) != 1 {
					t.Fatalf("got %d entries, want 1", len(entries))
				}

				e := entries[0]

				if e.Route != "/rates" {
					t.Errorf("Route = %q, want /rates", e.Route)
				}
				if e.Tag != "ThemePages.Rates" {
					t.Errorf("Tag = %q, want ThemePages.Rates", e.Tag)
				}
				if e.Kind != mdFileType {
					t.Errorf("Kind = %q, want %q", e.Kind, mdFileType)
				}
				if e.Title != "Server Rates" {
					t.Errorf("Title = %q, want %q (H1 derived)", e.Title, "Server Rates")
				}
			},
		},
		{
			name: "frontmatter title overrides h1",
			files: map[string]string{
				"info.md": "---\ntitle: Custom Title\n---\n# H1 Heading\n",
			},
			check: func(t *testing.T, entries []pageEntry) {
				if len(entries) != 1 {
					t.Fatalf("got %d entries, want 1", len(entries))
				}
				if entries[0].Title != "Custom Title" {
					t.Errorf("Title = %q, want %q (frontmatter override)", entries[0].Title, "Custom Title")
				}
			},
		},
		{
			name: "h1 overrides filename derived title",
			files: map[string]string{
				"info.md": "# H1 Heading\nbody\n",
			},
			check: func(t *testing.T, entries []pageEntry) {
				if len(entries) != 1 {
					t.Fatalf("got %d entries, want 1", len(entries))
				}
				if entries[0].Title != "H1 Heading" {
					t.Errorf("Title = %q, want %q (H1 override)", entries[0].Title, "H1 Heading")
				}
			},
		},
		{
			name: "filename derives title when no frontmatter and no h1",
			files: map[string]string{
				"server-info.md": "no heading\n",
			},
			check: func(t *testing.T, entries []pageEntry) {
				if len(entries) != 1 {
					t.Fatalf("got %d entries, want 1", len(entries))
				}
				if entries[0].Title == "" {
					t.Errorf("Title is empty, want non-empty filename-derived title")
				}
			},
		},
		{
			name: "underscore prefix file skipped",
			files: map[string]string{
				"_partial.md":  "partial content\n",
				"published.md": "# Published\n",
			},
			check: func(t *testing.T, entries []pageEntry) {
				if len(entries) != 1 {
					t.Fatalf("got %d entries, want 1 (_partial.md should be skipped)", len(entries))
				}
				if entries[0].Route != "/published" {
					t.Errorf("Route = %q, want /published", entries[0].Route)
				}
			},
		},
		{
			name: "non-md non-templ files ignored",
			files: map[string]string{
				"readme.txt": "ignore me\n",
				"page.md":    "# Page\n",
			},
			check: func(t *testing.T, entries []pageEntry) {
				if len(entries) != 1 {
					t.Fatalf("got %d entries, want 1 (txt should be ignored)", len(entries))
				}
				if entries[0].Route != "/page" {
					t.Errorf("Route = %q, want /page", entries[0].Route)
				}
			},
		},
		{
			name: "tag collision across dash and underscore variants rejected",
			files: map[string]string{
				"server-info.md": "# Dash\n",
				"server_info.md": "# Underscore\n",
			},
			wantErrPart: "ThemePages.ServerInfo",
		},
		{
			name: "empty tag suffix rejected",
			files: map[string]string{
				"-.md": "# Hyphen Only\n",
			},
			wantErrPart: "empty tag suffix",
		},
		{
			name: "leading digit tag rejected",
			files: map[string]string{
				"2foo.md": "# Digits First\n",
			},
			wantErrPart: "non-letter",
		},
		{
			name: "dynamic segment rejected",
			files: map[string]string{
				"[id].templ": "package pages\n\ntempl Id() {}\n",
			},
			wantErrPart: "dynamic segments not supported",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()

			for rel, content := range tt.files {
				full := filepath.Join(dir, rel)

				if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
					t.Fatalf("mkdir %s: %v", full, err)
				}

				if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
					t.Fatalf("write %s: %v", full, err)
				}
			}

			entries, err := collectThemePages(dir)

			if tt.wantErrPart != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil with entries=%#v", tt.wantErrPart, entries)
				}
				if !strings.Contains(err.Error(), tt.wantErrPart) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.wantErrPart)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			tt.check(t, entries)
		})
	}
}

func TestCollectThemePages_NestedAndUnderscoreDirsSkipped(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	files := map[string]string{
		"top.md":             "# Top\n",
		"events/summer.md":   "# Summer Event\n",
		"_internal/draft.md": "# Draft\n",
	}

	for rel, content := range files {
		full := filepath.Join(dir, rel)

		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", full, err)
		}

		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}

	entries, err := collectThemePages(dir)
	if err != nil {
		t.Fatalf("collectThemePages: %v", err)
	}

	gotRoutes := map[string]bool{}

	for _, e := range entries {
		gotRoutes[e.Route] = true
	}

	if !gotRoutes["/top"] {
		t.Errorf("missing /top route")
	}

	if !gotRoutes["/events/summer"] {
		t.Errorf("missing /events/summer route")
	}

	for route := range gotRoutes {
		if strings.Contains(route, "_internal") || strings.Contains(route, "draft") {
			t.Errorf("route %q should have been skipped (underscore dir)", route)
		}
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
