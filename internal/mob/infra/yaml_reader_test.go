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

func TestReadYAML_MissingFileReturnsErrNotExist(t *testing.T) {
	t.Parallel()

	_, err := ReadYAML(filepath.Join(t.TempDir(), "absent.yml"))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("err = %v, want fs.ErrNotExist", err)
	}
}

func TestReadYAML_EmptyFileReturnsNil(t *testing.T) {
	t.Parallel()

	path := writeTempFile(t, "empty.yml", nil)
	got, err := ReadYAML(path)
	if err != nil {
		t.Fatalf("ReadYAML: %v", err)
	}
	if got != nil {
		t.Errorf("got = %v, want nil for empty file", got)
	}
}

func TestReadYAML_MalformedYAMLReturnsError(t *testing.T) {
	t.Parallel()

	path := writeTempFile(t, "broken.yml", []byte("this: is: not: valid: yaml\n  -\n"))
	_, err := ReadYAML(path)
	if err == nil {
		t.Fatal("err = nil, want parse error")
	}
}

func TestReadYAML_ValidBodyParsesMultipleMobs(t *testing.T) {
	t.Parallel()

	source := []byte(`Header:
  Type: MOB_DB
  Version: 3
Body:
  - Id: 1002
    AegisName: PORING
    Name: Poring
    Level: 1
    Hp: 50
    BaseExp: 2
    JobExp: 1
    Race: Plant
    Element: Water
    ElementLevel: 1
    Size: Small
    Modes:
      CanMove: true
    Drops:
      - Item: Red_Potion
        Rate: 1000
  - Id: 1511
    AegisName: AMON_RA
    Name: Amon Ra
    Level: 88
    Hp: 1276000
    MvpExp: 528750
    Race: Demihuman
    Element: Fire
    Size: Large
    Modes:
      Mvp: true
    MvpDrops:
      - Item: Old_Card_Album
        Rate: 5500
`)
	path := writeTempFile(t, "mobs.yml", source)

	got, err := ReadYAML(path)
	if err != nil {
		t.Fatalf("ReadYAML: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}

	poring := got[0]
	if poring.ID != 1002 {
		t.Errorf("[0].ID = %d, want 1002", poring.ID)
	}
	if poring.AegisName != "PORING" {
		t.Errorf("[0].AegisName = %q", poring.AegisName)
	}
	if poring.Race != "Plant" {
		t.Errorf("[0].Race = %q", poring.Race)
	}
	if poring.Element != "Water" {
		t.Errorf("[0].Element = %q", poring.Element)
	}
	if poring.Size != "Small" {
		t.Errorf("[0].Size = %q", poring.Size)
	}
	if !poring.Modes["CanMove"] {
		t.Errorf("[0].Modes[CanMove] = false, want true")
	}
	if len(poring.Drops) != 1 || poring.Drops[0].Item != "Red_Potion" || poring.Drops[0].Rate != 1000 {
		t.Errorf("[0].Drops = %+v", poring.Drops)
	}

	amonra := got[1]
	if amonra.ID != 1511 || amonra.AegisName != "AMON_RA" {
		t.Errorf("[1] header mismatch: id=%d aegis=%q", amonra.ID, amonra.AegisName)
	}
	if amonra.MvpExp != 528750 {
		t.Errorf("[1].MvpExp = %d, want 528750", amonra.MvpExp)
	}
	if !amonra.Modes["Mvp"] {
		t.Errorf("[1].Modes[Mvp] = false, want true")
	}
	if len(amonra.MvpDrops) != 1 || amonra.MvpDrops[0].Item != "Old_Card_Album" {
		t.Errorf("[1].MvpDrops = %+v", amonra.MvpDrops)
	}
}

func TestReadYAML_MissingBodyReturnsNil(t *testing.T) {
	t.Parallel()

	source := []byte("Header:\n  Type: MOB_DB\n")
	path := writeTempFile(t, "header_only.yml", source)

	got, err := ReadYAML(path)
	if err != nil {
		t.Fatalf("ReadYAML: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len(got) = %d, want 0 (no Body key)", len(got))
	}
}
