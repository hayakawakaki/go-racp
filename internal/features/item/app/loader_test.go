package app

import (
	"path/filepath"
	"testing"

	"github.com/hayakawakaki/go-racp/internal/features/item/domain"
	"github.com/hayakawakaki/go-racp/internal/refdata"
)

func yamlGroup(t *testing.T, relatives ...string) refdata.SourceGroup {
	t.Helper()
	files := make([]string, len(relatives))
	for i, rel := range relatives {
		files[i] = absPath(t, rel)
	}

	return refdata.SourceGroup{Files: files}
}

func luaGroup(t *testing.T, relatives ...string) refdata.SourceGroup {
	t.Helper()

	return yamlGroup(t, relatives...)
}

func absPath(t *testing.T, relative string) string {
	t.Helper()
	abs, err := filepath.Abs(relative)
	if err != nil {
		t.Fatalf("filepath.Abs(%q): %v", relative, err)
	}

	return abs
}

func TestParseSources_BasicMerge(t *testing.T) {
	snap, err := ParseSources(Sources{
		YAML: yamlGroup(t, "testdata/single.yml"),
		Lua:  luaGroup(t, "testdata/iteminfo.lua"),
	})
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	if snap.SourceCount != 2 {
		t.Fatalf("SourceCount = %d, want 2", snap.SourceCount)
	}
	redPotion, ok := snap.ByID[501]
	if !ok {
		t.Fatalf("ByID[501] missing")
	}
	if redPotion.AegisName != "Red_Potion" {
		t.Errorf("AegisName = %q, want Red_Potion", redPotion.AegisName)
	}
	if redPotion.ClientName != "Red Potion" {
		t.Errorf("ClientName = %q, want %q", redPotion.ClientName, "Red Potion")
	}
	if len(redPotion.Description) != 5 {
		t.Fatalf("Description len = %d, want 5", len(redPotion.Description))
	}
	if redPotion.Image == "" {
		t.Errorf("Image empty, want lowercased resource name")
	}
	if redPotion.Type != domain.ItemTypeHealing {
		t.Errorf("Type = %v, want Healing", redPotion.Type)
	}
	sword, ok := snap.ByID[1101]
	if !ok {
		t.Fatalf("ByID[1101] missing")
	}
	if sword.WeaponLevel != 1 {
		t.Errorf("WeaponLevel = %d, want 1", sword.WeaponLevel)
	}
	if !sword.Refineable {
		t.Errorf("Refineable = false, want true")
	}
}

func TestParseSources_LastFileWins(t *testing.T) {
	snap, err := ParseSources(Sources{
		YAML: yamlGroup(t, "testdata/single.yml", "testdata/override.yml"),
		Lua:  luaGroup(t, "testdata/iteminfo.lua"),
	})
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	overridden, ok := snap.ByID[501]
	if !ok {
		t.Fatalf("ByID[501] missing")
	}
	if overridden.Buy != 9999 {
		t.Errorf("Buy = %d, want 9999 (override)", overridden.Buy)
	}
	if overridden.Name != "Pink Potion" {
		t.Errorf("Name = %q, want %q", overridden.Name, "Pink Potion")
	}
	if overridden.AegisName != "Pink_Potion" {
		t.Errorf("AegisName = %q, want %q", overridden.AegisName, "Pink_Potion")
	}
}

func TestParseSources_NoLuaForItem_UsesFallbacks(t *testing.T) {
	snap, err := ParseSources(Sources{
		YAML: yamlGroup(t, "testdata/single.yml"),
	})
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	sword, ok := snap.ByID[1101]
	if !ok {
		t.Fatalf("ByID[1101] missing")
	}
	if len(sword.Description) != 1 || sword.Description[0] != "No description." {
		t.Errorf("Description = %v, want [\"No description.\"]", sword.Description)
	}
	if sword.Image != "unknown" {
		t.Errorf("Image = %q, want unknown", sword.Image)
	}
	if sword.ClientName != sword.Name {
		t.Errorf("ClientName = %q, want fallback to Name %q", sword.ClientName, sword.Name)
	}
}

func TestParseSources_EmptyConfig_ReturnsEmptySnapshot(t *testing.T) {
	snap, err := ParseSources(Sources{})
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	if snap == nil {
		t.Fatal("snap is nil")
	}
	if snap.SourceCount != 0 {
		t.Errorf("SourceCount = %d, want 0", snap.SourceCount)
	}
}

func TestParseSources_MalformedYAML_ReturnsError(t *testing.T) {
	_, err := ParseSources(Sources{YAML: yamlGroup(t, "testdata/malformed.yml")})
	if err == nil {
		t.Fatal("err is nil, want a parse error")
	}
}

func TestParseSources_MalformedLua_SkipsSilently(t *testing.T) {
	snap, err := ParseSources(Sources{
		YAML: yamlGroup(t, "testdata/single.yml"),
		Lua:  luaGroup(t, "testdata/malformed.lua"),
	})
	if err != nil {
		t.Fatalf("ParseSources err = %v, want nil (regex parser tolerates malformed entries)", err)
	}
	if snap == nil || snap.SourceCount == 0 {
		t.Fatal("snapshot should still contain YAML items even when Lua entries can't be extracted")
	}
}
