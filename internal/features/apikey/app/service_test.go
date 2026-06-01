package app

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/apikey/domain"
)

func encodeAndHash(raw []byte) (rawKey string, hash []byte) {
	sum := sha256.Sum256(raw)

	return base64.RawURLEncoding.EncodeToString(raw), sum[:]
}

type touchCall struct {
	at time.Time
	id int64
}

type fakeRepo struct {
	createErr error
	listErr   error
	loadErr   error
	revokeErr error
	touched   chan touchCall
	created   []domain.APIKey
	active    []domain.APIKey
	revoked   []int64
	nextID    int64
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{touched: make(chan touchCall, 4), nextID: 1}
}

func (r *fakeRepo) Create(_ context.Context, key *domain.APIKey) error {
	if r.createErr != nil {
		return r.createErr
	}

	key.ID = r.nextID
	r.nextID++
	key.CreatedAt = time.Unix(0, 0).UTC()
	r.created = append(r.created, *key)

	return nil
}

func (r *fakeRepo) List(_ context.Context) ([]domain.APIKey, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}

	return r.created, nil
}

func (r *fakeRepo) Revoke(_ context.Context, id int64) error {
	if r.revokeErr != nil {
		return r.revokeErr
	}
	r.revoked = append(r.revoked, id)

	return nil
}

func (r *fakeRepo) LoadActive(_ context.Context) ([]domain.APIKey, error) {
	if r.loadErr != nil {
		return nil, r.loadErr
	}

	return r.active, nil
}

func (r *fakeRepo) TouchLastUsed(_ context.Context, id int64, at time.Time) error {
	r.touched <- touchCall{id: id, at: at}

	return nil
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestService(repo *fakeRepo) *Service {
	tiers := domain.NewTierSet([]domain.Tier{
		{Name: "Standard", RatePerMinute: 180, Burst: 180},
		{Name: "Elevated", RatePerMinute: 600, Burst: 600},
	})

	return NewService(repo, tiers, discardLogger())
}

func TestNewService_NilRepoPanics(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil repo, got none")
		}
	}()
	NewService(nil, domain.NewTierSet(nil), nil)
}

func TestNewService_NilLoggerUsesDefault(t *testing.T) {
	t.Parallel()
	svc := NewService(newFakeRepo(), domain.NewTierSet(nil), nil)
	if svc.logger == nil {
		t.Errorf("logger is nil after NewService(nil logger)")
	}
}

func TestService_Generate_Validation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		keyName    string
		tier       string
		wantFields []string
	}{
		{name: "empty name", keyName: "  ", tier: "Standard", wantFields: []string{"name"}},
		{name: "unknown tier", keyName: "deploy bot", tier: "Platinum", wantFields: []string{"tier"}},
		{name: "empty name and unknown tier", keyName: "", tier: "Platinum", wantFields: []string{"name", "tier"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := newTestService(newFakeRepo())

			raw, key, err := svc.Generate(context.Background(), tt.keyName, tt.tier)
			if raw != "" || key != nil {
				t.Errorf("Generate returned raw=%q key=%v on validation failure", raw, key)
			}

			var validation *domain.ValidationError
			if !errors.As(err, &validation) {
				t.Fatalf("err = %v, want *domain.ValidationError", err)
			}
			for _, field := range tt.wantFields {
				if _, ok := validation.Fields[field]; !ok {
					t.Errorf("missing field %q in %v", field, validation.Fields)
				}
			}
			if len(validation.Fields) != len(tt.wantFields) {
				t.Errorf("fields = %v, want %d entries", validation.Fields, len(tt.wantFields))
			}
		})
	}
}

func TestService_Generate_Success(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	svc := newTestService(repo)

	raw, key, err := svc.Generate(context.Background(), "  deploy bot  ", "Standard")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if key.ID != 1 {
		t.Errorf("key.ID = %d, want 1", key.ID)
	}
	if key.Name != "deploy bot" {
		t.Errorf("key.Name = %q, want trimmed %q", key.Name, "deploy bot")
	}
	if key.RateTier != "Standard" {
		t.Errorf("key.RateTier = %q, want Standard", key.RateTier)
	}
	if len(repo.created) != 1 {
		t.Fatalf("repo.created len = %d, want 1", len(repo.created))
	}

	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		t.Fatalf("raw key is not base64 RawURLEncoding: %v", err)
	}
	if len(decoded) != 32 {
		t.Errorf("decoded raw key len = %d, want 32", len(decoded))
	}
}

func TestService_Generate_ThenValidateRoundTrips(t *testing.T) {
	t.Parallel()
	svc := newTestService(newFakeRepo())

	raw, created, err := svc.Generate(context.Background(), "deploy bot", "Elevated")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	got, err := svc.Validate(context.Background(), raw)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if got.ID != created.ID || got.Name != "deploy bot" || got.RateTier != "Elevated" {
		t.Errorf("Validate returned %+v, want id=%d name=deploy bot tier=Elevated", got, created.ID)
	}
}

func TestService_Generate_RepoErrorPropagates(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	repo.createErr = errors.New("db down")
	svc := newTestService(repo)

	if _, _, err := svc.Generate(context.Background(), "deploy bot", "Standard"); err == nil {
		t.Fatal("Generate err = nil, want repo error")
	}
}

func TestService_Validate_Errors(t *testing.T) {
	t.Parallel()

	unknown := base64.RawURLEncoding.EncodeToString(make([]byte, 32))

	tests := []struct {
		name   string
		rawKey string
	}{
		{name: "not base64", rawKey: "not valid base64 !!!"},
		{name: "wrong length", rawKey: base64.RawURLEncoding.EncodeToString([]byte("short"))},
		{name: "well formed but unknown", rawKey: unknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := newTestService(newFakeRepo())

			key, err := svc.Validate(context.Background(), tt.rawKey)
			if key != nil {
				t.Errorf("Validate returned key %v, want nil", key)
			}
			if !errors.Is(err, domain.ErrKeyNotFound) {
				t.Errorf("err = %v, want ErrKeyNotFound", err)
			}
		})
	}
}

func TestService_Revoke_EvictsCache(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	svc := newTestService(repo)

	raw, created, err := svc.Generate(context.Background(), "deploy bot", "Standard")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if err := svc.Revoke(context.Background(), created.ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if len(repo.revoked) != 1 || repo.revoked[0] != created.ID {
		t.Errorf("repo.revoked = %v, want [%d]", repo.revoked, created.ID)
	}

	if _, err := svc.Validate(context.Background(), raw); !errors.Is(err, domain.ErrKeyNotFound) {
		t.Errorf("Validate after revoke err = %v, want ErrKeyNotFound", err)
	}
}

func TestService_Revoke_RepoErrorKeepsCache(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	svc := newTestService(repo)

	raw, created, err := svc.Generate(context.Background(), "deploy bot", "Standard")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	repo.revokeErr = errors.New("db down")
	if err := svc.Revoke(context.Background(), created.ID); err == nil {
		t.Fatal("Revoke err = nil, want repo error")
	}

	if _, err := svc.Validate(context.Background(), raw); err != nil {
		t.Errorf("Validate after failed revoke err = %v, want key still valid", err)
	}
}

func TestService_List(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	svc := newTestService(repo)

	if _, _, err := svc.Generate(context.Background(), "first", "Standard"); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	keys, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(keys) != 1 || keys[0].Name != "first" {
		t.Errorf("List = %+v, want one key named first", keys)
	}

	repo.listErr = errors.New("db down")
	if _, err := svc.List(context.Background()); err == nil {
		t.Fatal("List err = nil, want repo error")
	}
}

func TestService_Tiers_SortedByName(t *testing.T) {
	t.Parallel()
	svc := newTestService(newFakeRepo())

	tiers := svc.Tiers()
	if len(tiers) != 2 {
		t.Fatalf("Tiers len = %d, want 2", len(tiers))
	}
	if tiers[0].Name != "Elevated" || tiers[1].Name != "Standard" {
		t.Errorf("Tiers order = [%s %s], want [Elevated Standard]", tiers[0].Name, tiers[1].Name)
	}
}

func TestService_Warm_PopulatesCache(t *testing.T) {
	t.Parallel()

	raw := make([]byte, 32)
	for index := range raw {
		raw[index] = byte(index)
	}
	rawKey, hash := encodeAndHash(raw)

	repo := newFakeRepo()
	repo.active = []domain.APIKey{{ID: 7, Name: "warmed", RateTier: "Elevated", KeyHash: hash}}
	svc := newTestService(repo)

	if err := svc.Warm(context.Background()); err != nil {
		t.Fatalf("Warm: %v", err)
	}

	got, err := svc.Validate(context.Background(), rawKey)
	if err != nil {
		t.Fatalf("Validate after Warm: %v", err)
	}
	if got.ID != 7 || got.Name != "warmed" || got.RateTier != "Elevated" {
		t.Errorf("Validate = %+v, want id=7 name=warmed tier=Elevated", got)
	}
}

func TestService_Warm_RepoErrorPropagates(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	repo.loadErr = errors.New("db down")
	svc := newTestService(repo)

	if err := svc.Warm(context.Background()); err == nil {
		t.Fatal("Warm err = nil, want repo error")
	}
}

func TestService_Validate_TouchesLastUsed(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	svc := newTestService(repo)

	fixed := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return fixed }

	raw, created, err := svc.Generate(context.Background(), "deploy bot", "Standard")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if _, err := svc.Validate(context.Background(), raw); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	select {
	case touch := <-repo.touched:
		if touch.id != created.ID {
			t.Errorf("touched id = %d, want %d", touch.id, created.ID)
		}
		if !touch.at.Equal(fixed) {
			t.Errorf("touched at = %v, want %v", touch.at, fixed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("TouchLastUsed was not called within 2s")
	}
}

func TestService_Validate_SkipsTouchWithinInterval(t *testing.T) {
	t.Parallel()

	raw := make([]byte, 32)
	for index := range raw {
		raw[index] = byte(index)
	}
	rawKey, hash := encodeAndHash(raw)

	fixed := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	lastUsed := fixed.Add(-30 * time.Second)

	repo := newFakeRepo()
	repo.active = []domain.APIKey{{ID: 7, Name: "warmed", RateTier: "Standard", KeyHash: hash, LastUsedAt: &lastUsed}}
	svc := newTestService(repo)
	svc.now = func() time.Time { return fixed }

	if err := svc.Warm(context.Background()); err != nil {
		t.Fatalf("Warm: %v", err)
	}

	if _, err := svc.Validate(context.Background(), rawKey); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	select {
	case touch := <-repo.touched:
		t.Errorf("TouchLastUsed called within flush interval: %+v", touch)
	case <-time.After(100 * time.Millisecond):
	}
}
