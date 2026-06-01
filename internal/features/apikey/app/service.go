package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/apikey/domain"
)

const lastUsedFlushInterval = time.Minute

type cachedKey struct {
	name        string
	tier        string
	lastFlushed atomic.Int64
	id          int64
}

type Service struct {
	repo   domain.Repository
	logger *slog.Logger
	now    func() time.Time
	byHash map[string]*cachedKey
	tiers  domain.TierSet
	mu     sync.RWMutex
}

func NewService(repo domain.Repository, tiers domain.TierSet, logger *slog.Logger) *Service {
	if repo == nil {
		panic("apikey.NewService: repo must not be nil")
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &Service{
		repo:   repo,
		tiers:  tiers,
		logger: logger,
		now:    time.Now,
		byHash: make(map[string]*cachedKey),
	}
}

func (s *Service) Warm(ctx context.Context) error {
	keys, err := s.repo.LoadActive(ctx)
	if err != nil {
		return fmt.Errorf("app.Service.Warm: %w", err)
	}

	byHash := make(map[string]*cachedKey, len(keys))
	for _, key := range keys {
		entry := &cachedKey{
			id:   key.ID,
			name: key.Name,
			tier: key.RateTier,
		}
		if key.LastUsedAt != nil {
			entry.lastFlushed.Store(key.LastUsedAt.UnixNano())
		}
		byHash[string(key.KeyHash)] = entry
	}

	s.mu.Lock()
	s.byHash = byHash
	s.mu.Unlock()

	return nil
}

func (s *Service) Generate(ctx context.Context, name, tier string) (string, *domain.APIKey, error) {
	name = strings.TrimSpace(name)

	fields := domain.FieldErrors{}
	if name == "" {
		fields.Add("name", "name is required")
	}
	if !s.tiers.Has(tier) {
		fields.Add("tier", "unknown rate tier")
	}
	if fields.Has() {
		return "", nil, &domain.ValidationError{Fields: fields}
	}

	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", nil, fmt.Errorf("app.Service.Generate: %w", err)
	}

	hash := sha256.Sum256(raw[:])
	key := &domain.APIKey{
		KeyHash:  hash[:],
		Name:     name,
		RateTier: tier,
	}

	if err := s.repo.Create(ctx, key); err != nil {
		return "", nil, fmt.Errorf("app.Service.Generate: %w", err)
	}

	entry := &cachedKey{
		id:   key.ID,
		name: key.Name,
		tier: key.RateTier,
	}

	s.mu.Lock()
	s.byHash[string(hash[:])] = entry
	s.mu.Unlock()

	return base64.RawURLEncoding.EncodeToString(raw[:]), key, nil
}

func (s *Service) List(ctx context.Context) ([]domain.APIKey, error) {
	keys, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("app.Service.List: %w", err)
	}

	return keys, nil
}

func (s *Service) Tiers() []domain.Tier {
	return s.tiers.List()
}

func (s *Service) Revoke(ctx context.Context, id int64) error {
	if err := s.repo.Revoke(ctx, id); err != nil {
		return fmt.Errorf("app.Service.Revoke: %w", err)
	}

	s.mu.Lock()
	for hash, entry := range s.byHash {
		if entry.id == id {
			delete(s.byHash, hash)
			break
		}
	}
	s.mu.Unlock()

	return nil
}

func (s *Service) Validate(ctx context.Context, rawKey string) (*domain.APIKey, error) {
	hash, ok := decodeKeyToHash(rawKey)
	if !ok {
		return nil, domain.ErrKeyNotFound
	}

	s.mu.RLock()
	entry, found := s.byHash[string(hash[:])]
	s.mu.RUnlock()
	if !found {
		return nil, domain.ErrKeyNotFound
	}

	s.scheduleTouch(entry)

	return &domain.APIKey{ID: entry.id, Name: entry.name, RateTier: entry.tier}, nil
}

func (s *Service) scheduleTouch(entry *cachedKey) {
	now := s.now()

	previous := entry.lastFlushed.Load()
	if now.Sub(time.Unix(0, previous)) < lastUsedFlushInterval {
		return
	}
	if !entry.lastFlushed.CompareAndSwap(previous, now.UnixNano()) {
		return
	}

	id := entry.id
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := s.repo.TouchLastUsed(ctx, id, now); err != nil {
			s.logger.Warn("apikey touch last used failed", "id", id, "err", err)
		}
	}()
}

func decodeKeyToHash(rawKey string) ([32]byte, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(rawKey)
	if err != nil || len(raw) != 32 {
		return [32]byte{}, false
	}

	return sha256.Sum256(raw), true
}
