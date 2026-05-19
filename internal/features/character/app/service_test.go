package app

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/character/domain"
)

type updateLookCall struct {
	charID       int
	hair         int
	hairColor    int
	clothesColor int
}

type updateLocationCall struct {
	mapName string
	charID  int
	x       int
	y       int
}

type fakeCharRepo struct {
	byID                map[int]*domain.Character
	listByAccountHook   func(int) ([]domain.Character, error)
	getByIDHook         func(int) (*domain.Character, error)
	updateLookHook      func(int, int, int, int) error
	updateLocationHook  func(int, string, int, int) error
	updateLookCalls     []updateLookCall
	updateLocationCalls []updateLocationCall
	mu                  sync.Mutex
}

func newFakeCharRepo() *fakeCharRepo {
	return &fakeCharRepo{byID: map[int]*domain.Character{}}
}

func (f *fakeCharRepo) put(c domain.Character) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := c
	f.byID[c.ID] = &cp
}

func (f *fakeCharRepo) ListByAccount(_ context.Context, accountID int) ([]domain.Character, error) {
	if f.listByAccountHook != nil {
		return f.listByAccountHook(accountID)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]domain.Character, 0)
	for _, c := range f.byID {
		if c.AccountID == accountID {
			out = append(out, *c)
		}
	}

	return out, nil
}

func (f *fakeCharRepo) GetByID(_ context.Context, charID int) (*domain.Character, error) {
	if f.getByIDHook != nil {
		return f.getByIDHook(charID)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.byID[charID]
	if !ok {
		return nil, domain.ErrCharacterNotFound
	}
	cp := *c

	return &cp, nil
}

func (f *fakeCharRepo) UpdateLook(_ context.Context, charID, hair, hairColor, clothesColor int) error {
	if f.updateLookHook != nil {
		return f.updateLookHook(charID, hair, hairColor, clothesColor)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updateLookCalls = append(f.updateLookCalls, updateLookCall{charID, hair, hairColor, clothesColor})
	if c, ok := f.byID[charID]; ok {
		c.HairStyle = hair
		c.HairColor = hairColor
		c.ClothesColor = clothesColor
	}

	return nil
}

func (f *fakeCharRepo) UpdateLocation(_ context.Context, charID int, mapName string, x, y int) error {
	if f.updateLocationHook != nil {
		return f.updateLocationHook(charID, mapName, x, y)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updateLocationCalls = append(f.updateLocationCalls, updateLocationCall{mapName: mapName, charID: charID, x: x, y: y})
	if c, ok := f.byID[charID]; ok {
		c.CurrentMap = mapName
		c.CurrentX = x
		c.CurrentY = y
	}

	return nil
}

type cooldownKey struct {
	charID int
	t      domain.ChangeType
}

type recordCall struct {
	at     time.Time
	charID int
	t      domain.ChangeType
}

type fakeCooldowns struct {
	records        map[cooldownKey]time.Time
	recordHook     func(int, domain.ChangeType, time.Time) error
	mostRecentHook func(int, domain.ChangeType) (time.Time, error)
	recordCalls    []recordCall
	mu             sync.Mutex
}

func newFakeCooldowns() *fakeCooldowns {
	return &fakeCooldowns{records: map[cooldownKey]time.Time{}}
}

func (f *fakeCooldowns) Record(_ context.Context, charID int, t domain.ChangeType, at time.Time) error {
	if f.recordHook != nil {
		return f.recordHook(charID, t, at)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.records[cooldownKey{charID, t}] = at
	f.recordCalls = append(f.recordCalls, recordCall{at: at, charID: charID, t: t})

	return nil
}

func (f *fakeCooldowns) MostRecent(_ context.Context, charID int, t domain.ChangeType) (time.Time, error) {
	if f.mostRecentHook != nil {
		return f.mostRecentHook(charID, t)
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.records[cooldownKey{charID, t}], nil
}

func fixedNow() time.Time {
	return time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
}

func TestNewService_DefaultsNowToTimeNow(t *testing.T) {
	t.Parallel()
	svc := NewService(newFakeCharRepo(), newFakeCooldowns())
	if got := svc.Now(); time.Since(got) > time.Second {
		t.Errorf("default Now() = %v; not close to wall clock", got)
	}
}

func TestWithNow_NilFallsBackToTimeNow(t *testing.T) {
	t.Parallel()
	svc := NewService(newFakeCharRepo(), newFakeCooldowns(), WithNow(nil))
	if got := svc.Now(); time.Since(got) > time.Second {
		t.Errorf("WithNow(nil) Now() = %v; expected wall clock fallback", got)
	}
}

func TestWithNow_OverridesClock(t *testing.T) {
	t.Parallel()
	fixed := fixedNow()
	svc := NewService(newFakeCharRepo(), newFakeCooldowns(), WithNow(func() time.Time { return fixed }))
	if !svc.Now().Equal(fixed) {
		t.Errorf("Now() = %v, want %v", svc.Now(), fixed)
	}
}

func TestService_List(t *testing.T) {
	t.Parallel()

	t.Run("empty returns empty slice", func(t *testing.T) {
		t.Parallel()
		svc := NewService(newFakeCharRepo(), newFakeCooldowns())
		out, err := svc.List(context.Background(), 100)
		if err != nil {
			t.Fatalf("List err = %v", err)
		}
		if len(out) != 0 {
			t.Errorf("len(out) = %d, want 0", len(out))
		}
	})

	t.Run("returns dtos for owned characters with cooldown decorations", func(t *testing.T) {
		t.Parallel()
		repo := newFakeCharRepo()
		cds := newFakeCooldowns()
		repo.put(domain.Character{ID: 1, AccountID: 100, Slot: 0, Name: "kaki", JobID: 1, Zeny: 1234})
		repo.put(domain.Character{ID: 2, AccountID: 100, Slot: 1, Name: "crazyarashi", JobID: 4008})
		repo.put(domain.Character{ID: 3, AccountID: 200, Name: "other"})
		fixed := fixedNow()
		_ = cds.Record(context.Background(), 1, domain.ChangeTypeLook, fixed.Add(-time.Hour))

		svc := NewService(repo, cds,
			WithNow(func() time.Time { return fixed }),
			WithCooldowns(2*time.Hour, 30*time.Minute),
		)

		out, err := svc.List(context.Background(), 100)
		if err != nil {
			t.Fatalf("List err = %v", err)
		}
		if len(out) != 2 {
			t.Fatalf("len(out) = %d, want 2", len(out))
		}

		byID := map[int]CharacterDTO{}
		for _, dto := range out {
			byID[dto.ID] = dto
		}
		k, ok := byID[1]
		if !ok {
			t.Fatalf("missing dto for id 1; got %+v", byID)
		}
		if k.JobName != "Swordsman" {
			t.Errorf("dto.JobName = %q, want Swordsman", k.JobName)
		}
		wantLookUntil := fixed.Add(-time.Hour).Add(2 * time.Hour)
		if !k.LookCDUntil.Equal(wantLookUntil) {
			t.Errorf("LookCDUntil = %v, want %v", k.LookCDUntil, wantLookUntil)
		}
		if !k.LocCDUntil.IsZero() {
			t.Errorf("LocCDUntil = %v, want zero (never recorded)", k.LocCDUntil)
		}

		a, ok := byID[2]
		if !ok {
			t.Fatalf("missing dto for id 2")
		}
		if a.JobName != "Lord Knight" {
			t.Errorf("dto.JobName = %q, want Lord Knight", a.JobName)
		}
		if !a.LookCDUntil.IsZero() || !a.LocCDUntil.IsZero() {
			t.Errorf("expected both cooldowns zero for fresh character; got look=%v loc=%v", a.LookCDUntil, a.LocCDUntil)
		}
	})

	t.Run("repo error wraps", func(t *testing.T) {
		t.Parallel()
		repo := newFakeCharRepo()
		repo.listByAccountHook = func(int) ([]domain.Character, error) { return nil, errors.New("boom") }
		svc := NewService(repo, newFakeCooldowns())

		_, err := svc.List(context.Background(), 100)
		if err == nil || !strings.Contains(err.Error(), "app.Service.List") {
			t.Errorf("not wrapped: %v", err)
		}
	})

	t.Run("cooldown lookup error wraps", func(t *testing.T) {
		t.Parallel()
		repo := newFakeCharRepo()
		repo.put(domain.Character{ID: 1, AccountID: 100})
		cds := newFakeCooldowns()
		cds.mostRecentHook = func(int, domain.ChangeType) (time.Time, error) {
			return time.Time{}, errors.New("boom")
		}
		svc := NewService(repo, cds)

		_, err := svc.List(context.Background(), 100)
		if err == nil || !strings.Contains(err.Error(), "app.Service.List") {
			t.Errorf("not wrapped: %v", err)
		}
	})
}

func TestService_Get(t *testing.T) {
	t.Parallel()

	t.Run("happy path returns decorated dto", func(t *testing.T) {
		t.Parallel()
		repo := newFakeCharRepo()
		cds := newFakeCooldowns()
		fixed := fixedNow()
		repo.put(domain.Character{ID: 1, AccountID: 100, Name: "kaki", JobID: 4252, Zeny: 50000})
		_ = cds.Record(context.Background(), 1, domain.ChangeTypeLocation, fixed.Add(-15*time.Minute))
		svc := NewService(repo, cds,
			WithNow(func() time.Time { return fixed }),
			WithCooldowns(time.Hour, time.Hour),
		)

		dto, err := svc.Get(context.Background(), 100, 1)
		if err != nil {
			t.Fatalf("Get err = %v", err)
		}
		if dto.ID != 1 || dto.Name != "kaki" {
			t.Errorf("dto identity wrong: %+v", dto)
		}
		if dto.JobName != "Dragon Knight" {
			t.Errorf("dto.JobName = %q, want Dragon Knight", dto.JobName)
		}
		wantLocUntil := fixed.Add(-15 * time.Minute).Add(time.Hour)
		if !dto.LocCDUntil.Equal(wantLocUntil) {
			t.Errorf("LocCDUntil = %v, want %v", dto.LocCDUntil, wantLocUntil)
		}
		if !dto.LookCDUntil.IsZero() {
			t.Errorf("LookCDUntil = %v, want zero", dto.LookCDUntil)
		}
	})

	t.Run("wrong owner returns ErrNotOwner unwrapped", func(t *testing.T) {
		t.Parallel()
		repo := newFakeCharRepo()
		repo.put(domain.Character{ID: 1, AccountID: 100})
		svc := NewService(repo, newFakeCooldowns())

		_, err := svc.Get(context.Background(), 999, 1)
		if !errors.Is(err, domain.ErrNotOwner) {
			t.Fatalf("err = %v, want ErrNotOwner", err)
		}
		if strings.Contains(err.Error(), "app.Service.Get") {
			t.Errorf("ErrNotOwner must pass through unwrapped, got %q", err.Error())
		}
	})

	t.Run("repo not found wraps", func(t *testing.T) {
		t.Parallel()
		svc := NewService(newFakeCharRepo(), newFakeCooldowns())
		_, err := svc.Get(context.Background(), 100, 1)
		if err == nil || !strings.Contains(err.Error(), "app.Service.Get") {
			t.Errorf("not wrapped: %v", err)
		}
	})

	t.Run("cooldown lookup error wraps", func(t *testing.T) {
		t.Parallel()
		repo := newFakeCharRepo()
		repo.put(domain.Character{ID: 1, AccountID: 100})
		cds := newFakeCooldowns()
		cds.mostRecentHook = func(int, domain.ChangeType) (time.Time, error) {
			return time.Time{}, errors.New("boom")
		}
		svc := NewService(repo, cds)

		_, err := svc.Get(context.Background(), 100, 1)
		if err == nil || !strings.Contains(err.Error(), "app.Service.Get") {
			t.Errorf("not wrapped: %v", err)
		}
	})
}

func TestService_ResetLook(t *testing.T) {
	t.Parallel()

	t.Run("happy path resets look fields and records cooldown", func(t *testing.T) {
		t.Parallel()
		repo := newFakeCharRepo()
		cds := newFakeCooldowns()
		fixed := fixedNow()
		repo.put(domain.Character{ID: 1, AccountID: 100, HairStyle: 5, HairColor: 6, ClothesColor: 7})
		svc := NewService(repo, cds,
			WithNow(func() time.Time { return fixed }),
			WithCooldowns(24*time.Hour, time.Hour),
		)

		if err := svc.ResetLook(context.Background(), 100, 1); err != nil {
			t.Fatalf("ResetLook err = %v", err)
		}
		if len(repo.updateLookCalls) != 1 {
			t.Fatalf("UpdateLook calls = %d, want 1", len(repo.updateLookCalls))
		}
		got := repo.updateLookCalls[0]
		if got != (updateLookCall{charID: 1, hair: 0, hairColor: 0, clothesColor: 0}) {
			t.Errorf("UpdateLook call = %+v, want (1,0,0,0)", got)
		}
		if at := cds.records[cooldownKey{1, domain.ChangeTypeLook}]; !at.Equal(fixed) {
			t.Errorf("cooldown recorded at %v, want %v", at, fixed)
		}
	})

	t.Run("wrong owner returns ErrNotOwner", func(t *testing.T) {
		t.Parallel()
		repo := newFakeCharRepo()
		repo.put(domain.Character{ID: 1, AccountID: 100})
		svc := NewService(repo, newFakeCooldowns())

		err := svc.ResetLook(context.Background(), 999, 1)
		if !errors.Is(err, domain.ErrNotOwner) {
			t.Errorf("err = %v, want ErrNotOwner", err)
		}
	})

	t.Run("character online returns ErrCharacterOnline", func(t *testing.T) {
		t.Parallel()
		repo := newFakeCharRepo()
		repo.put(domain.Character{ID: 1, AccountID: 100, Online: true})
		svc := NewService(repo, newFakeCooldowns())

		err := svc.ResetLook(context.Background(), 100, 1)
		if !errors.Is(err, domain.ErrCharacterOnline) {
			t.Errorf("err = %v, want ErrCharacterOnline", err)
		}
	})

	t.Run("cooldown active returns ErrCooldown", func(t *testing.T) {
		t.Parallel()
		repo := newFakeCharRepo()
		cds := newFakeCooldowns()
		fixed := fixedNow()
		repo.put(domain.Character{ID: 1, AccountID: 100})
		_ = cds.Record(context.Background(), 1, domain.ChangeTypeLook, fixed.Add(-time.Minute))
		svc := NewService(repo, cds,
			WithNow(func() time.Time { return fixed }),
			WithCooldowns(24*time.Hour, time.Hour),
		)

		err := svc.ResetLook(context.Background(), 100, 1)
		if !errors.Is(err, domain.ErrCooldown) {
			t.Errorf("err = %v, want ErrCooldown", err)
		}
		if len(repo.updateLookCalls) != 0 {
			t.Errorf("UpdateLook should not be called when on cooldown, got %d calls", len(repo.updateLookCalls))
		}
	})

	t.Run("cooldown boundary equal to window succeeds", func(t *testing.T) {
		t.Parallel()
		repo := newFakeCharRepo()
		cds := newFakeCooldowns()
		fixed := fixedNow()
		window := 24 * time.Hour
		repo.put(domain.Character{ID: 1, AccountID: 100})
		_ = cds.Record(context.Background(), 1, domain.ChangeTypeLook, fixed.Add(-window))
		svc := NewService(repo, cds,
			WithNow(func() time.Time { return fixed }),
			WithCooldowns(window, time.Hour),
		)

		if err := svc.ResetLook(context.Background(), 100, 1); err != nil {
			t.Errorf("ResetLook at exactly window boundary should succeed; got %v", err)
		}
	})

	t.Run("ignores location cooldown when looking at look reset", func(t *testing.T) {
		t.Parallel()
		repo := newFakeCharRepo()
		cds := newFakeCooldowns()
		fixed := fixedNow()
		repo.put(domain.Character{ID: 1, AccountID: 100})
		_ = cds.Record(context.Background(), 1, domain.ChangeTypeLocation, fixed.Add(-time.Second))
		svc := NewService(repo, cds,
			WithNow(func() time.Time { return fixed }),
			WithCooldowns(24*time.Hour, time.Hour),
		)

		if err := svc.ResetLook(context.Background(), 100, 1); err != nil {
			t.Errorf("ResetLook err = %v; location cooldown must not block look reset", err)
		}
	})

	t.Run("guardMutation wraps GetByID error", func(t *testing.T) {
		t.Parallel()
		repo := newFakeCharRepo()
		repo.getByIDHook = func(int) (*domain.Character, error) { return nil, errors.New("boom") }
		svc := NewService(repo, newFakeCooldowns())

		err := svc.ResetLook(context.Background(), 100, 1)
		if err == nil || !strings.Contains(err.Error(), "app.Service.guardMutation") {
			t.Errorf("not wrapped via guardMutation: %v", err)
		}
	})

	t.Run("ResetLook wraps UpdateLook error", func(t *testing.T) {
		t.Parallel()
		repo := newFakeCharRepo()
		repo.put(domain.Character{ID: 1, AccountID: 100})
		repo.updateLookHook = func(int, int, int, int) error { return errors.New("boom") }
		svc := NewService(repo, newFakeCooldowns())

		err := svc.ResetLook(context.Background(), 100, 1)
		if err == nil || !strings.Contains(err.Error(), "app.Service.ResetLook") {
			t.Errorf("not wrapped: %v", err)
		}
	})

	t.Run("ResetLook wraps cooldown Record error", func(t *testing.T) {
		t.Parallel()
		repo := newFakeCharRepo()
		repo.put(domain.Character{ID: 1, AccountID: 100})
		cds := newFakeCooldowns()
		cds.recordHook = func(int, domain.ChangeType, time.Time) error { return errors.New("boom") }
		svc := NewService(repo, cds)

		err := svc.ResetLook(context.Background(), 100, 1)
		if err == nil || !strings.Contains(err.Error(), "app.Service.ResetLook") {
			t.Errorf("not wrapped: %v", err)
		}
	})
}

func TestService_ResetLocation(t *testing.T) {
	t.Parallel()

	defaultLoc := DefaultLocation{Map: "prontera", X: 156, Y: 191}

	t.Run("happy path writes default location and records cooldown", func(t *testing.T) {
		t.Parallel()
		repo := newFakeCharRepo()
		cds := newFakeCooldowns()
		fixed := fixedNow()
		repo.put(domain.Character{ID: 1, AccountID: 100, CurrentMap: "geffen", CurrentX: 50, CurrentY: 50})
		svc := NewService(repo, cds,
			WithNow(func() time.Time { return fixed }),
			WithCooldowns(time.Hour, time.Hour),
			WithDefaultLocation(defaultLoc),
		)

		if err := svc.ResetLocation(context.Background(), 100, 1); err != nil {
			t.Fatalf("ResetLocation err = %v", err)
		}
		if len(repo.updateLocationCalls) != 1 {
			t.Fatalf("UpdateLocation calls = %d, want 1", len(repo.updateLocationCalls))
		}
		got := repo.updateLocationCalls[0]
		want := updateLocationCall{mapName: "prontera", charID: 1, x: 156, y: 191}
		if got != want {
			t.Errorf("UpdateLocation call = %+v, want %+v", got, want)
		}
		if at := cds.records[cooldownKey{1, domain.ChangeTypeLocation}]; !at.Equal(fixed) {
			t.Errorf("cooldown recorded at %v, want %v", at, fixed)
		}
	})

	t.Run("wrong owner returns ErrNotOwner", func(t *testing.T) {
		t.Parallel()
		repo := newFakeCharRepo()
		repo.put(domain.Character{ID: 1, AccountID: 100})
		svc := NewService(repo, newFakeCooldowns(), WithDefaultLocation(defaultLoc))

		err := svc.ResetLocation(context.Background(), 999, 1)
		if !errors.Is(err, domain.ErrNotOwner) {
			t.Errorf("err = %v, want ErrNotOwner", err)
		}
	})

	t.Run("character online returns ErrCharacterOnline", func(t *testing.T) {
		t.Parallel()
		repo := newFakeCharRepo()
		repo.put(domain.Character{ID: 1, AccountID: 100, Online: true})
		svc := NewService(repo, newFakeCooldowns(), WithDefaultLocation(defaultLoc))

		err := svc.ResetLocation(context.Background(), 100, 1)
		if !errors.Is(err, domain.ErrCharacterOnline) {
			t.Errorf("err = %v, want ErrCharacterOnline", err)
		}
	})

	t.Run("cooldown active returns ErrCooldown", func(t *testing.T) {
		t.Parallel()
		repo := newFakeCharRepo()
		cds := newFakeCooldowns()
		fixed := fixedNow()
		repo.put(domain.Character{ID: 1, AccountID: 100})
		_ = cds.Record(context.Background(), 1, domain.ChangeTypeLocation, fixed.Add(-time.Minute))
		svc := NewService(repo, cds,
			WithNow(func() time.Time { return fixed }),
			WithCooldowns(time.Hour, time.Hour),
			WithDefaultLocation(defaultLoc),
		)

		err := svc.ResetLocation(context.Background(), 100, 1)
		if !errors.Is(err, domain.ErrCooldown) {
			t.Errorf("err = %v, want ErrCooldown", err)
		}
		if len(repo.updateLocationCalls) != 0 {
			t.Errorf("UpdateLocation should not be called when on cooldown")
		}
	})

	t.Run("ignores look cooldown when resetting location", func(t *testing.T) {
		t.Parallel()
		repo := newFakeCharRepo()
		cds := newFakeCooldowns()
		fixed := fixedNow()
		repo.put(domain.Character{ID: 1, AccountID: 100})
		_ = cds.Record(context.Background(), 1, domain.ChangeTypeLook, fixed.Add(-time.Second))
		svc := NewService(repo, cds,
			WithNow(func() time.Time { return fixed }),
			WithCooldowns(24*time.Hour, time.Hour),
			WithDefaultLocation(defaultLoc),
		)

		if err := svc.ResetLocation(context.Background(), 100, 1); err != nil {
			t.Errorf("ResetLocation err = %v; look cooldown must not block location reset", err)
		}
	})

	t.Run("ResetLocation wraps UpdateLocation error", func(t *testing.T) {
		t.Parallel()
		repo := newFakeCharRepo()
		repo.put(domain.Character{ID: 1, AccountID: 100})
		repo.updateLocationHook = func(int, string, int, int) error { return errors.New("boom") }
		svc := NewService(repo, newFakeCooldowns(), WithDefaultLocation(defaultLoc))

		err := svc.ResetLocation(context.Background(), 100, 1)
		if err == nil || !strings.Contains(err.Error(), "app.Service.ResetLocation") {
			t.Errorf("not wrapped: %v", err)
		}
	})

	t.Run("ResetLocation wraps cooldown Record error", func(t *testing.T) {
		t.Parallel()
		repo := newFakeCharRepo()
		repo.put(domain.Character{ID: 1, AccountID: 100})
		cds := newFakeCooldowns()
		cds.recordHook = func(int, domain.ChangeType, time.Time) error { return errors.New("boom") }
		svc := NewService(repo, cds, WithDefaultLocation(defaultLoc))

		err := svc.ResetLocation(context.Background(), 100, 1)
		if err == nil || !strings.Contains(err.Error(), "app.Service.ResetLocation") {
			t.Errorf("not wrapped: %v", err)
		}
	})
}
