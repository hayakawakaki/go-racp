package app

import (
	"path/filepath"
	"testing"

	"github.com/hayakawakaki/go-racp/internal/features/mob/domain"
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

func absPath(t *testing.T, relative string) string {
	t.Helper()
	abs, err := filepath.Abs(relative)
	if err != nil {
		t.Fatalf("filepath.Abs(%q): %v", relative, err)
	}

	return abs
}

func TestParseSources_BasicLoad(t *testing.T) {
	snap, err := ParseSources(Sources{YAML: yamlGroup(t, "testdata/single.yml")})
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	if snap.SourceCount != 2 {
		t.Fatalf("SourceCount = %d, want 2", snap.SourceCount)
	}
	poring, ok := snap.ByID[1002]
	if !ok {
		t.Fatalf("ByID[1002] missing")
	}
	if poring.AegisName != "PORING" {
		t.Errorf("AegisName = %q, want PORING", poring.AegisName)
	}
	if poring.AegisLower != "poring" {
		t.Errorf("AegisLower = %q, want poring", poring.AegisLower)
	}
	if poring.Race != domain.RacePlant {
		t.Errorf("Race = %v, want Plant", poring.Race)
	}
	if poring.Element != domain.ElementWater {
		t.Errorf("Element = %v, want Water", poring.Element)
	}
	if poring.Size != domain.SizeSmall {
		t.Errorf("Size = %v, want Small", poring.Size)
	}
	if !poring.Modes.Has(domain.ModeCanMove) {
		t.Errorf("Modes missing CanMove")
	}

	amon, ok := snap.ByID[1511]
	if !ok {
		t.Fatalf("ByID[1511] missing")
	}
	if !amon.IsMVP() {
		t.Errorf("Amon Ra IsMVP = false, want true")
	}
}

func TestParseSources_AegisLookupIsLowercased(t *testing.T) {
	snap, err := ParseSources(Sources{YAML: yamlGroup(t, "testdata/single.yml")})
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	if _, ok := snap.ByAegis["poring"]; !ok {
		t.Errorf("ByAegis[poring] missing, got keys: %v", keys(snap.ByAegis))
	}
	if _, ok := snap.ByAegis["PORING"]; ok {
		t.Errorf("ByAegis[PORING] present, expected lowercased keys only")
	}
}

func TestParseSources_DroppedByIndexedAndSortedByRate(t *testing.T) {
	snap, err := ParseSources(Sources{YAML: yamlGroup(t, "testdata/multi_drops.yml")})
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	entries, ok := snap.DroppedBy["red_potion"]
	if !ok {
		t.Fatalf("DroppedBy[red_potion] missing")
	}
	if len(entries) != 2 {
		t.Fatalf("entries len = %d, want 2", len(entries))
	}
	if entries[0].Rate < entries[1].Rate {
		t.Errorf("entries not sorted by rate desc: %+v", entries)
	}
	if entries[0].MobID != 1031 {
		t.Errorf("top entry MobID = %d, want 1031 (higher rate)", entries[0].MobID)
	}
}

func TestParseSources_EmptyItemAegisDropIsSkipped(t *testing.T) {
	snap, err := ParseSources(Sources{YAML: yamlGroup(t, "testdata/multi_drops.yml")})
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	if _, ok := snap.DroppedBy[""]; ok {
		t.Errorf("DroppedBy contains empty key, want skipped")
	}
}

func TestParseSources_MvpDropFlagged(t *testing.T) {
	snap, err := ParseSources(Sources{YAML: yamlGroup(t, "testdata/single.yml")})
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	entries, ok := snap.DroppedBy["old_card_album"]
	if !ok || len(entries) != 1 {
		t.Fatalf("DroppedBy[old_card_album] = %v, want 1 entry", entries)
	}
	if !entries[0].IsMVP {
		t.Errorf("entry.IsMVP = false, want true for MvpDrops")
	}
}

func TestParseSources_LastFileWinsByID(t *testing.T) {
	snap, err := ParseSources(Sources{
		YAML: yamlGroup(t, "testdata/single.yml", "testdata/override.yml"),
	})
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	poring, ok := snap.ByID[1002]
	if !ok {
		t.Fatalf("ByID[1002] missing")
	}
	if poring.AegisName != "PORING_OVERRIDE" {
		t.Errorf("AegisName = %q, want PORING_OVERRIDE", poring.AegisName)
	}
	if poring.Name != "Pink Poring" {
		t.Errorf("Name = %q, want Pink Poring", poring.Name)
	}
	if poring.HP != 99 {
		t.Errorf("HP = %d, want 99", poring.HP)
	}
}

func TestParseSources_EmptyConfigReturnsEmptySnapshot(t *testing.T) {
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

func TestParseSources_MalformedYAMLReturnsError(t *testing.T) {
	_, err := ParseSources(Sources{YAML: yamlGroup(t, "testdata/malformed.yml")})
	if err == nil {
		t.Fatal("err = nil, want a parse error")
	}
}

func TestParseSources_MissingFileSkipsSilently(t *testing.T) {
	snap, err := ParseSources(Sources{
		YAML: yamlGroup(t, "testdata/does_not_exist.yml"),
	})
	if err != nil {
		t.Fatalf("ParseSources err = %v, want nil (missing files are warned and skipped)", err)
	}
	if snap == nil {
		t.Fatal("snap is nil")
	}
	if snap.SourceCount != 0 {
		t.Errorf("SourceCount = %d, want 0", snap.SourceCount)
	}
}

func keys(m map[string]*domain.Mob) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	return out
}
