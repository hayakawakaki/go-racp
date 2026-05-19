package app

import (
	"testing"

	"github.com/hayakawakaki/go-racp/internal/features/mob/domain"
)

func TestToDTO_NilReturnsZeroValue(t *testing.T) {
	t.Parallel()

	got := ToDTO(nil)
	if got.ID != 0 || got.AegisName != "" || got.Name != "" || got.Race != "" ||
		got.Element != "" || got.Size != "" || got.IsMVP || got.Drops != nil || got.MvpDrops != nil {
		t.Errorf("ToDTO(nil) = %+v, want zero MobDTO", got)
	}
}

func TestToDTO_MapsCoreFields(t *testing.T) {
	t.Parallel()

	mob := &domain.Mob{
		ID:           1002,
		AegisName:    "PORING",
		Name:         "Poring",
		JapaneseName: "ポリン",
		Level:        1,
		HP:           50,
		BaseExp:      2,
		JobExp:       1,
		Race:         domain.RacePlant,
		Element:      domain.ElementWater,
		Size:         domain.SizeSmall,
	}

	got := ToDTO(mob)

	if got.ID != 1002 || got.AegisName != "PORING" || got.Name != "Poring" {
		t.Errorf("identity fields = %+v", got)
	}
	if got.JapaneseName != "ポリン" {
		t.Errorf("JapaneseName = %q", got.JapaneseName)
	}
	if got.Level != 1 || got.HP != 50 || got.BaseExp != 2 || got.JobExp != 1 {
		t.Errorf("stats = lvl=%d hp=%d be=%d je=%d", got.Level, got.HP, got.BaseExp, got.JobExp)
	}
	if got.Race != "Plant" || got.Element != "Water" || got.Size != "Small" {
		t.Errorf("enums = race=%q element=%q size=%q", got.Race, got.Element, got.Size)
	}
	if got.Sprite == "" {
		t.Errorf("Sprite = empty, want non-empty (ResolveSprite always returns something)")
	}
	if got.IsMVP {
		t.Errorf("IsMVP = true, want false for non-mvp mob")
	}
}

func TestToDTO_MVPMobMarkedAsMVP(t *testing.T) {
	t.Parallel()

	mob := &domain.Mob{ID: 1511, AegisName: "AMON_RA", MvpExp: 528750}

	got := ToDTO(mob)

	if !got.IsMVP {
		t.Errorf("IsMVP = false, want true when MvpExp > 0")
	}
	if got.MvpExp != 528750 {
		t.Errorf("MvpExp = %d, want 528750", got.MvpExp)
	}
}

func TestToDTO_MapsDropsAndMvpDrops(t *testing.T) {
	t.Parallel()

	mob := &domain.Mob{
		ID:        1002,
		AegisName: "PORING",
		Drops: []domain.MobDrop{
			{ItemAegis: "Red_Potion", Rate: 1000},
			{ItemAegis: "Jellopy", Rate: 9000, StealProtected: true},
		},
		MvpDrops: []domain.MobDrop{
			{ItemAegis: "Old_Card_Album", Rate: 5500, RandomOptionGroup: "MVP"},
		},
	}

	got := ToDTO(mob)

	if len(got.Drops) != 2 {
		t.Fatalf("Drops len = %d, want 2", len(got.Drops))
	}
	if got.Drops[0].ItemAegis != "Red_Potion" || got.Drops[0].Rate != 1000 {
		t.Errorf("Drops[0] = %+v", got.Drops[0])
	}
	if !got.Drops[1].StealProtected {
		t.Errorf("Drops[1].StealProtected = false, want true")
	}
	if len(got.MvpDrops) != 1 {
		t.Fatalf("MvpDrops len = %d, want 1", len(got.MvpDrops))
	}
	if got.MvpDrops[0].RandomOptionGroup != "MVP" {
		t.Errorf("MvpDrops[0].RandomOptionGroup = %q, want MVP", got.MvpDrops[0].RandomOptionGroup)
	}
}

func TestToDTO_EmptyDropsRendersAsNil(t *testing.T) {
	t.Parallel()

	got := ToDTO(&domain.Mob{ID: 1, AegisName: "X"})
	if got.Drops != nil {
		t.Errorf("Drops = %v, want nil", got.Drops)
	}
	if got.MvpDrops != nil {
		t.Errorf("MvpDrops = %v, want nil", got.MvpDrops)
	}
}

func TestToDTO_ModesRenderedThroughDisplay(t *testing.T) {
	t.Parallel()

	var modes domain.ModeSet
	modes.Set(domain.ModeMvp)
	modes.Set(domain.ModeAggressive)
	mob := &domain.Mob{ID: 1, Modes: modes}

	got := ToDTO(mob)

	if len(got.Modes) != 2 {
		t.Fatalf("Modes len = %d, want 2", len(got.Modes))
	}
	want := []string{"Aggressive", "MVP"}
	for i, name := range want {
		if got.Modes[i] != name {
			t.Errorf("Modes[%d] = %q, want %q", i, got.Modes[i], name)
		}
	}
}
