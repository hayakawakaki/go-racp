package infra

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func writeTempFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}

	return path
}

func TestReadLua_MissingFileReturnsErrNotExist(t *testing.T) {
	t.Parallel()

	_, err := ReadLua(filepath.Join(t.TempDir(), "absent.lua"))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("err = %v, want fs.ErrNotExist", err)
	}
}

func TestReadLua_EmptyFileReturnsEmptyMap(t *testing.T) {
	t.Parallel()

	path := writeTempFile(t, "empty.lua", nil)
	got, err := ReadLua(path)
	if err != nil {
		t.Fatalf("ReadLua: %v", err)
	}
	if got == nil {
		t.Fatalf("map is nil, want non-nil empty map")
	}
	if len(got) != 0 {
		t.Errorf("len(got) = %d, want 0", len(got))
	}
}

func TestReadLua_ValidIteminfoParsesAllFields(t *testing.T) {
	t.Parallel()

	source := `tbl = {
		[501] = {
			identifiedDisplayName = "Red Potion",
			identifiedResourceName = "red_potion",
			identifiedDescriptionName = {
				"A bottle of potion.",
				"Heals HP."
			},
		},
		[1101] = {
			identifiedDisplayName = "Sword",
			identifiedResourceName = "sword",
			identifiedDescriptionName = {
				"A basic sword."
			},
		},
	}`

	path := writeTempFile(t, "iteminfo.lua", []byte(source))
	got, err := ReadLua(path)
	if err != nil {
		t.Fatalf("ReadLua: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}

	red, ok := got[501]
	if !ok {
		t.Fatalf("entry 501 missing")
	}
	if red.DisplayName != "Red Potion" {
		t.Errorf("501.DisplayName = %q, want %q", red.DisplayName, "Red Potion")
	}
	if red.Resource != "red_potion" {
		t.Errorf("501.Resource = %q, want %q", red.Resource, "red_potion")
	}
	if len(red.Description) != 2 || red.Description[0] != "A bottle of potion." || red.Description[1] != "Heals HP." {
		t.Errorf("501.Description = %v", red.Description)
	}

	sword, ok := got[1101]
	if !ok {
		t.Fatalf("entry 1101 missing")
	}
	if sword.DisplayName != "Sword" {
		t.Errorf("1101.DisplayName = %q, want %q", sword.DisplayName, "Sword")
	}
}

func TestReadLua_MalformedEntryIsSkipped(t *testing.T) {
	t.Parallel()

	source := `tbl = { [1] = { unclosed`
	path := writeTempFile(t, "broken.lua", []byte(source))

	got, err := ReadLua(path)
	if err != nil {
		t.Fatalf("ReadLua err = %v, want nil (regex parser tolerates malformed entries)", err)
	}
	if len(got) != 0 {
		t.Errorf("len(got) = %d, want 0 (no matching closing brace)", len(got))
	}
}

func TestReadLua_MixedValidAndInvalidIDs(t *testing.T) {
	t.Parallel()

	source := `tbl = {
		[banana] = {
			identifiedDisplayName = "Non-numeric ID",
		},
		[501] = {
			identifiedDisplayName = "Valid",
			identifiedResourceName = "valid",
			identifiedDescriptionName = {
				"ok"
			},
		},
		[99999999999999999999] = {
			identifiedDisplayName = "Overflowing ID",
		},
	}`
	path := writeTempFile(t, "partial.lua", []byte(source))

	got, err := ReadLua(path)
	if err != nil {
		t.Fatalf("ReadLua: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1 (only 501 should survive)", len(got))
	}
	if _, ok := got[501]; !ok {
		t.Errorf("501 missing")
	}
}

func TestParseIteminfo_NoEntries(t *testing.T) {
	t.Parallel()

	got := parseIteminfo("nothing matches here")
	if len(got) != 0 {
		t.Errorf("len(got) = %d, want 0", len(got))
	}
}

func TestParseIteminfo_ExtractsOnlyDeclaredFields(t *testing.T) {
	t.Parallel()

	source := `[999] = {
		identifiedDisplayName = "Only Name",
	}`
	got := parseIteminfo(source)
	entry, ok := got[999]
	if !ok {
		t.Fatalf("entry 999 missing")
	}
	if entry.DisplayName != "Only Name" {
		t.Errorf("DisplayName = %q, want %q", entry.DisplayName, "Only Name")
	}
	if entry.Resource != "" {
		t.Errorf("Resource = %q, want empty (field absent)", entry.Resource)
	}
	if entry.Description != nil {
		t.Errorf("Description = %v, want nil (field absent)", entry.Description)
	}
}

func TestParseIteminfo_HandlesNestedBraces(t *testing.T) {
	t.Parallel()

	source := `[10] = {
		identifiedDisplayName = "Has Nested",
		identifiedResourceName = "nested",
		identifiedDescriptionName = {
			"first line",
			"second line"
		},
		costume = false,
		nested = { inner = "value" },
	}`
	got := parseIteminfo(source)
	entry, ok := got[10]
	if !ok {
		t.Fatalf("entry 10 missing; brace matcher failed")
	}
	if entry.DisplayName != "Has Nested" {
		t.Errorf("DisplayName = %q", entry.DisplayName)
	}
	if len(entry.Description) != 2 {
		t.Errorf("Description = %v, want 2 lines", entry.Description)
	}
}

func TestFindMatchingBrace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
		open   int
		want   int
	}{
		{name: "simple pair", source: "{}", open: 0, want: 1},
		{name: "nested pair", source: "{ { } }", open: 0, want: 6},
		{name: "no match", source: "{ unclosed", open: 0, want: -1},
		{name: "string contains braces", source: `{ "}" }`, open: 0, want: 6},
		{name: "escaped quote inside string", source: `{ "a\"b" }`, open: 0, want: 9},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := findMatchingBrace(tt.source, tt.open); got != tt.want {
				t.Errorf("findMatchingBrace(%q, %d) = %d, want %d", tt.source, tt.open, got, tt.want)
			}
		})
	}
}

func TestDecodeToUTF8_PassesThroughValidUTF8(t *testing.T) {
	t.Parallel()

	input := []byte("Red Potion 빨간포션")
	got, err := decodeToUTF8(input)
	if err != nil {
		t.Fatalf("decodeToUTF8: %v", err)
	}
	if got != string(input) {
		t.Errorf("got %q, want %q (unchanged)", got, string(input))
	}
}

func TestDecodeToUTF8_DecodesEUCKR(t *testing.T) {
	t.Parallel()

	input := []byte{0xB0, 0xA1}
	got, err := decodeToUTF8(input)
	if err != nil {
		t.Fatalf("decodeToUTF8: %v", err)
	}
	if got != "가" {
		t.Errorf("decodeToUTF8(EUC-KR 0xB0 0xA1) = %q, want \"가\"", got)
	}
}

func TestExtractStrings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		block string
		want  []string
	}{
		{name: "empty block", block: "", want: nil},
		{name: "no strings", block: "foo bar", want: nil},
		{name: "single string", block: `"hello"`, want: []string{"hello"}},
		{name: "multiple strings across lines", block: "\"a\",\n\"b\",\n\"c\"", want: []string{"a", "b", "c"}},
		{name: "empty string is preserved", block: `""`, want: []string{""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractStrings(tt.block)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d (got=%v)", len(got), len(tt.want), got)
			}
			for index := range got {
				if got[index] != tt.want[index] {
					t.Errorf("got[%d] = %q, want %q", index, got[index], tt.want[index])
				}
			}
		})
	}
}
