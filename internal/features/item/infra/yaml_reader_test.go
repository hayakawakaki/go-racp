package infra

import (
	"errors"
	"io/fs"
	"path/filepath"
	"testing"
)

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

func TestReadYAML_ValidBodyParsesMultipleItems(t *testing.T) {
	t.Parallel()

	source := []byte(`Header:
  Type: ITEM_DB
  Version: 1
Body:
  - Id: 501
    AegisName: Red_Potion
    Name: Red Potion
    Type: Healing
    Buy: 10
    Weight: 70
  - Id: 1101
    AegisName: Sword
    Name: Sword
    Type: Weapon
    SubType: 1hSword
    Buy: 100
    Weight: 500
    Attack: 25
    Slots: 3
    WeaponLevel: 1
    Refineable: true
    Locations:
      Right_Hand: true
    Jobs:
      Swordman: true
      Knight: true
`)
	path := writeTempFile(t, "items.yml", source)

	got, err := ReadYAML(path)
	if err != nil {
		t.Fatalf("ReadYAML: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}

	red := got[0]
	if red.ID != 501 {
		t.Errorf("[0].ID = %d, want 501", red.ID)
	}
	if red.AegisName != "Red_Potion" {
		t.Errorf("[0].AegisName = %q", red.AegisName)
	}
	if red.Name != "Red Potion" {
		t.Errorf("[0].Name = %q", red.Name)
	}
	if red.Type != "Healing" {
		t.Errorf("[0].Type = %q", red.Type)
	}
	if red.Buy != 10 {
		t.Errorf("[0].Buy = %d", red.Buy)
	}
	if red.Weight != 70 {
		t.Errorf("[0].Weight = %d", red.Weight)
	}

	sword := got[1]
	if sword.ID != 1101 {
		t.Errorf("[1].ID = %d, want 1101", sword.ID)
	}
	if sword.SubType != "1hSword" {
		t.Errorf("[1].SubType = %q", sword.SubType)
	}
	if sword.Attack != 25 {
		t.Errorf("[1].Attack = %d, want 25", sword.Attack)
	}
	if sword.Slots != 3 {
		t.Errorf("[1].Slots = %d, want 3", sword.Slots)
	}
	if !sword.Refineable {
		t.Errorf("[1].Refineable = false, want true")
	}
	if !sword.Locations["Right_Hand"] {
		t.Errorf("[1].Locations[Right_Hand] = false, want true")
	}
	if !sword.Jobs["Swordman"] || !sword.Jobs["Knight"] {
		t.Errorf("[1].Jobs = %v, want Swordman+Knight enabled", sword.Jobs)
	}
}

func TestReadYAML_MissingBodyReturnsNil(t *testing.T) {
	t.Parallel()

	source := []byte("Header:\n  Type: ITEM_DB\n")
	path := writeTempFile(t, "header_only.yml", source)

	got, err := ReadYAML(path)
	if err != nil {
		t.Fatalf("ReadYAML: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len(got) = %d, want 0 (no Body key)", len(got))
	}
}
