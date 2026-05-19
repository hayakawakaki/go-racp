package app

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/item/domain"
	"github.com/hayakawakaki/go-racp/internal/refdata"
)

func newFixtureSnapshot(t *testing.T, items ...*domain.Item) *domain.Snapshot {
	t.Helper()
	snap := &domain.Snapshot{
		LoadedAt:    time.Now(),
		ByID:        map[int]*domain.Item{},
		ByName:      map[string]*domain.Item{},
		Sorted:      items,
		SourceCount: len(items),
	}
	for _, item := range items {
		snap.ByID[item.ID] = item
		snap.ByName[item.AegisName] = item
	}

	return snap
}

func TestService_Get_EmptySnapshot(t *testing.T) {
	service := NewServiceWithSnapshot(domain.EmptySnapshot(), nil)
	_, err := service.Get(context.Background(), 501)
	if !errors.Is(err, domain.ErrEmptySnapshot) {
		t.Fatalf("Get err = %v, want ErrEmptySnapshot", err)
	}
}

func TestService_Get_NotFound(t *testing.T) {
	snap := newFixtureSnapshot(t, &domain.Item{ID: 501, AegisName: "Red_Potion"})
	service := NewServiceWithSnapshot(snap, nil)
	_, err := service.Get(context.Background(), 999)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Get err = %v, want ErrNotFound", err)
	}
}

func TestService_Get_Found(t *testing.T) {
	snap := newFixtureSnapshot(t, &domain.Item{ID: 501, AegisName: "Red_Potion", Name: "Red Potion"})
	service := NewServiceWithSnapshot(snap, nil)
	item, err := service.Get(context.Background(), 501)
	if err != nil {
		t.Fatalf("Get err = %v", err)
	}
	if item.ID != 501 {
		t.Errorf("ID = %d, want 501", item.ID)
	}
}

func TestService_LookupByAegis_EmptyAegis(t *testing.T) {
	snap := newFixtureSnapshot(t, &domain.Item{ID: 501, AegisName: "Red_Potion"})
	service := NewServiceWithSnapshot(snap, nil)
	if got := service.LookupByAegis(""); got != nil {
		t.Errorf("LookupByAegis(\"\") = %v, want nil", got)
	}
}

func TestService_LookupByAegis_EmptySnapshot(t *testing.T) {
	service := NewServiceWithSnapshot(domain.EmptySnapshot(), nil)
	if got := service.LookupByAegis("Red_Potion"); got != nil {
		t.Errorf("LookupByAegis = %v, want nil for empty snapshot", got)
	}
}

func TestService_LookupByAegis_NotFound(t *testing.T) {
	snap := newFixtureSnapshot(t, &domain.Item{ID: 501, AegisName: "Red_Potion"})
	service := NewServiceWithSnapshot(snap, nil)
	if got := service.LookupByAegis("Unknown_Item"); got != nil {
		t.Errorf("LookupByAegis = %v, want nil for unknown aegis", got)
	}
}

func TestService_LookupByAegis_Found(t *testing.T) {
	want := &domain.Item{ID: 501, AegisName: "Red_Potion", Name: "Red Potion"}
	snap := newFixtureSnapshot(t, want)
	service := NewServiceWithSnapshot(snap, nil)
	got := service.LookupByAegis("Red_Potion")
	if got == nil {
		t.Fatal("LookupByAegis = nil, want item")
	}
	if got.ID != 501 || got.AegisName != "Red_Potion" {
		t.Errorf("got = %+v, want id=501 aegis=Red_Potion", got)
	}
}

func TestService_List_Pagination(t *testing.T) {
	items := make([]*domain.Item, 0, 45)
	for index := 1; index <= 45; index++ {
		items = append(items, &domain.Item{ID: index, AegisName: "Item", Name: "Item"})
	}
	snap := newFixtureSnapshot(t, items...)
	service := NewServiceWithSnapshot(snap, nil)

	page, err := service.List(context.Background(), ListQuery{Page: 2, PerPage: 20})
	if err != nil {
		t.Fatalf("List err = %v", err)
	}
	if len(page.Items) != 20 {
		t.Errorf("len(Items) = %d, want 20", len(page.Items))
	}
	if page.Items[0].ID != 21 {
		t.Errorf("first ID = %d, want 21", page.Items[0].ID)
	}
	if page.TotalPages != 3 {
		t.Errorf("TotalPages = %d, want 3", page.TotalPages)
	}
}

func TestService_List_FilterByQuery(t *testing.T) {
	snap := newFixtureSnapshot(t,
		&domain.Item{ID: 501, AegisName: "Red_Potion", AegisNameLower: "red_potion", Name: "Red", ClientName: "Red Potion", ClientNameLower: "red potion"},
		&domain.Item{ID: 502, AegisName: "Blue_Potion", AegisNameLower: "blue_potion", Name: "Blue", ClientName: "Blue Potion", ClientNameLower: "blue potion"},
	)
	service := NewServiceWithSnapshot(snap, nil)

	page, err := service.List(context.Background(), ListQuery{Page: 1, PerPage: 20, Query: "blue"})
	if err != nil {
		t.Fatalf("List err = %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].ID != 502 {
		t.Fatalf("filtered = %v, want only Blue_Potion", page.Items)
	}
}

func TestService_List_FilterByQueryNumericID(t *testing.T) {
	snap := newFixtureSnapshot(t,
		&domain.Item{ID: 501, AegisName: "Red_Potion"},
		&domain.Item{ID: 502, AegisName: "Blue_Potion"},
	)
	service := NewServiceWithSnapshot(snap, nil)

	page, err := service.List(context.Background(), ListQuery{Page: 1, PerPage: 20, Query: "501"})
	if err != nil {
		t.Fatalf("List err = %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].ID != 501 {
		t.Fatalf("filtered = %v, want only id 501", page.Items)
	}
}

func TestService_List_FilterByType(t *testing.T) {
	snap := newFixtureSnapshot(t,
		&domain.Item{ID: 501, AegisName: "Red_Potion", Type: domain.ItemTypeHealing},
		&domain.Item{ID: 1101, AegisName: "Sword", Type: domain.ItemTypeWeapon},
	)
	service := NewServiceWithSnapshot(snap, nil)

	page, err := service.List(context.Background(), ListQuery{Page: 1, PerPage: 20, Type: domain.ItemTypeWeapon})
	if err != nil {
		t.Fatalf("List err = %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].ID != 1101 {
		t.Fatalf("filtered = %v, want only Sword", page.Items)
	}
}

func TestService_Reload_FailurePreservesPrior(t *testing.T) {
	initial := newFixtureSnapshot(t, &domain.Item{ID: 501, AegisName: "Red_Potion"})
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
	item, err := service.Get(context.Background(), 501)
	if err != nil {
		t.Fatalf("Get after failed reload: %v", err)
	}
	if item.AegisName != "Red_Potion" {
		t.Errorf("item still served? AegisName = %q, want Red_Potion", item.AegisName)
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
