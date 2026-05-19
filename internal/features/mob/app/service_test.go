package app

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/mob/domain"
	"github.com/hayakawakaki/go-racp/internal/platform/refdata"
)

func newFixtureSnapshot(t *testing.T, mobs ...*domain.Mob) *domain.Snapshot {
	t.Helper()
	snap := &domain.Snapshot{
		LoadedAt:    time.Now(),
		ByID:        map[int]*domain.Mob{},
		ByAegis:     map[string]*domain.Mob{},
		DroppedBy:   map[string][]domain.DropOf{},
		Sorted:      mobs,
		SourceCount: len(mobs),
	}
	for _, mob := range mobs {
		snap.ByID[mob.ID] = mob
		snap.ByAegis[mob.AegisLower] = mob
	}

	return snap
}

func TestService_Get_EmptySnapshot(t *testing.T) {
	service := NewServiceWithSnapshot(domain.EmptySnapshot(), nil)
	_, err := service.Get(context.Background(), 1002)
	if !errors.Is(err, domain.ErrEmptySnapshot) {
		t.Fatalf("Get err = %v, want ErrEmptySnapshot", err)
	}
}

func TestService_Get_NotFound(t *testing.T) {
	snap := newFixtureSnapshot(t, &domain.Mob{ID: 1002, AegisName: "PORING", AegisLower: "poring"})
	service := NewServiceWithSnapshot(snap, nil)
	_, err := service.Get(context.Background(), 9999)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Get err = %v, want ErrNotFound", err)
	}
}

func TestService_Get_Found(t *testing.T) {
	snap := newFixtureSnapshot(t, &domain.Mob{ID: 1002, AegisName: "PORING", AegisLower: "poring", Name: "Poring"})
	service := NewServiceWithSnapshot(snap, nil)
	mob, err := service.Get(context.Background(), 1002)
	if err != nil {
		t.Fatalf("Get err = %v", err)
	}
	if mob.ID != 1002 {
		t.Errorf("ID = %d, want 1002", mob.ID)
	}
}

func TestService_Snapshot_NilFallsBackToEmpty(t *testing.T) {
	t.Parallel()

	service := NewService(nil)
	snap := service.Snapshot()
	if snap == nil {
		t.Fatal("Snapshot() = nil, want EmptySnapshot")
	}
	if snap.SourceCount != 0 {
		t.Errorf("SourceCount = %d, want 0", snap.SourceCount)
	}
}

func TestService_List_Pagination(t *testing.T) {
	mobs := make([]*domain.Mob, 0, 45)
	for index := 1; index <= 45; index++ {
		mobs = append(mobs, &domain.Mob{ID: index, AegisName: "Mob", AegisLower: "mob", Name: "Mob", NameLower: "mob"})
	}
	snap := newFixtureSnapshot(t, mobs...)
	service := NewServiceWithSnapshot(snap, nil)

	page, err := service.List(context.Background(), ListQuery{Page: 2, PerPage: 20})
	if err != nil {
		t.Fatalf("List err = %v", err)
	}
	if len(page.Mobs) != 20 {
		t.Errorf("len(Mobs) = %d, want 20", len(page.Mobs))
	}
	if page.Mobs[0].ID != 21 {
		t.Errorf("first ID = %d, want 21", page.Mobs[0].ID)
	}
	if page.TotalPages != 3 {
		t.Errorf("TotalPages = %d, want 3", page.TotalPages)
	}
}

func TestService_List_DefaultsApplied(t *testing.T) {
	t.Parallel()

	mobs := []*domain.Mob{{ID: 1, AegisName: "A", AegisLower: "a", Name: "A", NameLower: "a"}}
	service := NewServiceWithSnapshot(newFixtureSnapshot(t, mobs...), nil)

	page, err := service.List(context.Background(), ListQuery{})
	if err != nil {
		t.Fatalf("List err = %v", err)
	}
	if page.Page != 1 {
		t.Errorf("Page = %d, want 1", page.Page)
	}
	if page.PerPage != DefaultPerPage {
		t.Errorf("PerPage = %d, want %d", page.PerPage, DefaultPerPage)
	}
}

func TestService_List_PageBeyondLastIsEmpty(t *testing.T) {
	t.Parallel()

	mobs := []*domain.Mob{{ID: 1, AegisName: "A", AegisLower: "a", Name: "A", NameLower: "a"}}
	service := NewServiceWithSnapshot(newFixtureSnapshot(t, mobs...), nil)

	page, err := service.List(context.Background(), ListQuery{Page: 99, PerPage: 10})
	if err != nil {
		t.Fatalf("List err = %v", err)
	}
	if len(page.Mobs) != 0 {
		t.Errorf("len(Mobs) = %d, want 0 for page beyond last", len(page.Mobs))
	}
	if page.Total != 1 {
		t.Errorf("Total = %d, want 1", page.Total)
	}
}

func TestService_List_FilterByQueryName(t *testing.T) {
	t.Parallel()

	snap := newFixtureSnapshot(t,
		&domain.Mob{ID: 1002, AegisName: "PORING", AegisLower: "poring", Name: "Poring", NameLower: "poring"},
		&domain.Mob{ID: 1063, AegisName: "LUNATIC", AegisLower: "lunatic", Name: "Lunatic", NameLower: "lunatic"},
	)
	service := NewServiceWithSnapshot(snap, nil)

	page, err := service.List(context.Background(), ListQuery{Page: 1, PerPage: 20, Query: "lunatic"})
	if err != nil {
		t.Fatalf("List err = %v", err)
	}
	if len(page.Mobs) != 1 || page.Mobs[0].ID != 1063 {
		t.Fatalf("filtered = %v, want only Lunatic", page.Mobs)
	}
}

func TestService_List_FilterByNumericID(t *testing.T) {
	t.Parallel()

	snap := newFixtureSnapshot(t,
		&domain.Mob{ID: 1002, AegisName: "PORING", AegisLower: "poring", NameLower: "poring"},
		&domain.Mob{ID: 1063, AegisName: "LUNATIC", AegisLower: "lunatic", NameLower: "lunatic"},
	)
	service := NewServiceWithSnapshot(snap, nil)

	page, err := service.List(context.Background(), ListQuery{Page: 1, PerPage: 20, Query: "1002"})
	if err != nil {
		t.Fatalf("List err = %v", err)
	}
	if len(page.Mobs) != 1 || page.Mobs[0].ID != 1002 {
		t.Fatalf("filtered = %v, want only id 1002", page.Mobs)
	}
}

func TestService_List_EmptyQueryReturnsAll(t *testing.T) {
	t.Parallel()

	snap := newFixtureSnapshot(t,
		&domain.Mob{ID: 1, AegisName: "A", AegisLower: "a", NameLower: "a"},
		&domain.Mob{ID: 2, AegisName: "B", AegisLower: "b", NameLower: "b"},
	)
	service := NewServiceWithSnapshot(snap, nil)

	page, err := service.List(context.Background(), ListQuery{Page: 1, PerPage: 20})
	if err != nil {
		t.Fatalf("List err = %v", err)
	}
	if len(page.Mobs) != 2 {
		t.Errorf("len(Mobs) = %d, want 2", len(page.Mobs))
	}
}

func TestService_WhoDrops_EmptyInputs(t *testing.T) {
	t.Parallel()

	service := NewServiceWithSnapshot(domain.EmptySnapshot(), nil)
	if got := service.WhoDrops("Red_Potion"); got != nil {
		t.Errorf("empty snapshot WhoDrops = %v, want nil", got)
	}
	if got := service.WhoDrops(""); got != nil {
		t.Errorf("empty aegis WhoDrops = %v, want nil", got)
	}
}

func TestService_WhoDrops_ReturnsMatches(t *testing.T) {
	t.Parallel()

	snap := newFixtureSnapshot(t, &domain.Mob{ID: 1002, AegisName: "PORING", AegisLower: "poring"})
	snap.DroppedBy["red_potion"] = []domain.DropOf{
		{MobID: 1002, MobAegis: "PORING", MobName: "Poring", Rate: 1000},
	}
	service := NewServiceWithSnapshot(snap, nil)

	got := service.WhoDrops("Red_Potion")
	if len(got) != 1 || got[0].MobID != 1002 {
		t.Errorf("WhoDrops = %+v, want one entry for mob 1002", got)
	}
}

func TestService_WhoDrops_LookupIsCaseInsensitive(t *testing.T) {
	t.Parallel()

	snap := newFixtureSnapshot(t, &domain.Mob{ID: 1, AegisName: "PORING", AegisLower: "poring"})
	snap.DroppedBy["red_potion"] = []domain.DropOf{{MobID: 1, MobName: "Poring", Rate: 1}}
	service := NewServiceWithSnapshot(snap, nil)

	if got := service.WhoDrops("RED_POTION"); len(got) != 1 {
		t.Errorf("uppercase lookup len = %d, want 1", len(got))
	}
	if got := service.WhoDrops("red_potion"); len(got) != 1 {
		t.Errorf("lowercase lookup len = %d, want 1", len(got))
	}
}

func TestService_WhoDrops_UnknownAegisReturnsNil(t *testing.T) {
	t.Parallel()

	snap := newFixtureSnapshot(t, &domain.Mob{ID: 1, AegisName: "PORING", AegisLower: "poring"})
	snap.DroppedBy["red_potion"] = []domain.DropOf{{MobID: 1, MobName: "Poring", Rate: 1}}
	service := NewServiceWithSnapshot(snap, nil)

	if got := service.WhoDrops("Unknown_Item"); got != nil {
		t.Errorf("unknown aegis WhoDrops = %v, want nil", got)
	}
}

func TestService_Reload_NilLoaderReturnsParseError(t *testing.T) {
	t.Parallel()

	service := NewService(nil)
	err := service.Reload(context.Background())
	if !errors.Is(err, refdata.ErrParseFailed) {
		t.Errorf("err = %v, want ErrParseFailed", err)
	}
	if service.Status().LastError == "" {
		t.Errorf("LastError empty, want recorded failure")
	}
}

func TestService_Reload_NilSnapshotReturnsParseError(t *testing.T) {
	t.Parallel()

	loader := func() (*domain.Snapshot, error) { return nil, nil }
	service := NewService(loader)
	err := service.Reload(context.Background())
	if !errors.Is(err, refdata.ErrParseFailed) {
		t.Errorf("err = %v, want ErrParseFailed", err)
	}
}

func TestService_Reload_SuccessUpdatesSnapshotAndStatus(t *testing.T) {
	t.Parallel()

	next := newFixtureSnapshot(t, &domain.Mob{ID: 1, AegisName: "X", AegisLower: "x"})
	loader := func() (*domain.Snapshot, error) { return next, nil }
	service := NewService(loader)

	if err := service.Reload(context.Background()); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	status := service.Status()
	if status.MobsLoaded != 1 {
		t.Errorf("MobsLoaded = %d, want 1", status.MobsLoaded)
	}
	if status.LastReload == "" {
		t.Errorf("LastReload empty after success")
	}
	if status.LastError != "" {
		t.Errorf("LastError = %q, want empty", status.LastError)
	}
}

func TestService_Reload_FailurePreservesPrior(t *testing.T) {
	initial := newFixtureSnapshot(t, &domain.Mob{ID: 1002, AegisName: "PORING", AegisLower: "poring"})
	var calls atomic.Int32
	loader := func() (*domain.Snapshot, error) {
		number := calls.Add(1)
		if number == 1 {
			return initial, nil
		}

		return nil, errors.New("synthetic parse failure")
	}
	service := NewService(loader)
	if err := service.Reload(context.Background()); err != nil {
		t.Fatalf("first Reload: %v", err)
	}
	err := service.Reload(context.Background())
	if err == nil {
		t.Fatal("second Reload err = nil, want parse failure")
	}
	mob, err := service.Get(context.Background(), 1002)
	if err != nil {
		t.Fatalf("Get after failed reload: %v", err)
	}
	if mob.AegisName != "PORING" {
		t.Errorf("AegisName = %q, want PORING (prior snapshot preserved)", mob.AegisName)
	}
}

func TestService_Reload_TryLockBlocksSecondCall(t *testing.T) {
	gate := make(chan struct{})
	release := make(chan struct{})
	loader := func() (*domain.Snapshot, error) {
		gate <- struct{}{}
		<-release

		return domain.EmptySnapshot(), nil
	}
	service := NewService(loader)

	var firstErr, secondErr error
	var wg sync.WaitGroup
	wg.Go(func() {
		firstErr = service.Reload(context.Background())
	})
	<-gate

	secondErr = service.Reload(context.Background())
	close(release)
	wg.Wait()

	if firstErr != nil {
		t.Errorf("firstErr = %v, want nil", firstErr)
	}
	if !errors.Is(secondErr, refdata.ErrReloadConflict) {
		t.Errorf("secondErr = %v, want ErrReloadConflict", secondErr)
	}
}

func TestNewServiceWithSnapshot_StampsLastReloadAtBoot(t *testing.T) {
	t.Parallel()

	before := time.Now()
	service := NewServiceWithSnapshot(domain.EmptySnapshot(), nil)
	after := time.Now()

	status := service.Status()
	if status.LastReload == "" {
		t.Fatalf("LastReload empty after construction")
	}
	parsed, err := time.Parse(time.RFC3339, status.LastReload)
	if err != nil {
		t.Fatalf("LastReload %q not RFC3339: %v", status.LastReload, err)
	}
	if parsed.Before(before.Truncate(time.Second)) || parsed.After(after.Add(time.Second)) {
		t.Errorf("LastReload %v outside [%v, %v]", parsed, before, after)
	}
	if status.LastError != "" {
		t.Errorf("LastError = %q, want empty", status.LastError)
	}
}
