package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hayakawakaki/go-racp/internal/mob/domain"
)

type LoaderFunc func() (*domain.Snapshot, error)

//nolint:govet // singleton, alignment over readability
type Service struct {
	loader     LoaderFunc
	snap       atomic.Pointer[domain.Snapshot]
	reloadMu   sync.Mutex
	statusMu   sync.RWMutex
	lastReload time.Time
	lastError  string
}

func NewService(loader LoaderFunc) *Service {
	return &Service{loader: loader}
}

func NewServiceWithSnapshot(snap *domain.Snapshot, loader LoaderFunc) *Service {
	service := NewService(loader)
	service.snap.Store(snap)

	return service
}

func (s *Service) Snapshot() *domain.Snapshot {
	snap := s.snap.Load()
	if snap == nil {
		return domain.EmptySnapshot()
	}

	return snap
}

func (s *Service) Get(_ context.Context, id int) (*domain.Mob, error) {
	snap := s.Snapshot()
	if snap.SourceCount == 0 {
		return nil, domain.ErrEmptySnapshot
	}
	mob, ok := snap.ByID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}

	return mob, nil
}

func (s *Service) List(_ context.Context, query ListQuery) (MobPage, error) {
	snap := s.Snapshot()
	if query.PerPage <= 0 {
		query.PerPage = DefaultPerPage
	}
	if query.Page <= 0 {
		query.Page = 1
	}

	filtered := filterMobs(snap.Sorted, query)
	total := len(filtered)
	totalPages := (total + query.PerPage - 1) / query.PerPage
	start := (query.Page - 1) * query.PerPage
	end := start + query.PerPage
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	return MobPage{
		Mobs:       filtered[start:end],
		Total:      total,
		Page:       query.Page,
		PerPage:    query.PerPage,
		TotalPages: totalPages,
	}, nil
}

func filterMobs(mobs []*domain.Mob, query ListQuery) []*domain.Mob {
	if query.Query == "" {
		return mobs
	}
	needle := strings.ToLower(strings.TrimSpace(query.Query))
	var asID int
	if parsed, err := strconv.Atoi(needle); err == nil {
		asID = parsed
	}

	out := make([]*domain.Mob, 0, len(mobs))
	for _, mob := range mobs {
		if !matchesQuery(mob, needle, asID) {
			continue
		}
		out = append(out, mob)
	}

	return out
}

func matchesQuery(mob *domain.Mob, needle string, asID int) bool {
	if asID > 0 && mob.ID == asID {
		return true
	}
	if strings.Contains(mob.AegisLower, needle) {
		return true
	}
	if strings.Contains(mob.NameLower, needle) {
		return true
	}

	return false
}

func (s *Service) WhoDrops(itemAegis string) []domain.DropOf {
	if itemAegis == "" {
		return nil
	}
	snap := s.Snapshot()
	if snap.SourceCount == 0 || len(snap.DroppedBy) == 0 {
		return nil
	}
	entries, ok := snap.DroppedBy[strings.ToLower(itemAegis)]
	if !ok || len(entries) == 0 {
		return nil
	}
	out := make([]domain.DropOf, len(entries))
	copy(out, entries)

	return out
}

func (s *Service) Reload(_ context.Context) error {
	if !s.reloadMu.TryLock() {
		return domain.ErrReloadConflict
	}
	defer s.reloadMu.Unlock()

	if s.loader == nil {
		err := fmt.Errorf("app.Service.Reload: %w", domain.ErrParseFailed)
		s.recordFailure(err)

		return err
	}
	snap, err := s.loader()
	if err != nil {
		s.recordFailure(err)

		return fmt.Errorf("app.Service.Reload: %w", err)
	}

	s.snap.Store(snap)
	s.recordSuccess()

	return nil
}

func (s *Service) recordSuccess() {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	s.lastReload = time.Now()
	s.lastError = ""
}

func (s *Service) recordFailure(err error) {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	s.lastError = err.Error()
}

func (s *Service) Status() ServiceStatus {
	s.statusMu.RLock()
	defer s.statusMu.RUnlock()
	snap := s.Snapshot()
	lastReload := ""
	if !s.lastReload.IsZero() {
		lastReload = s.lastReload.Format(time.RFC3339)
	}

	return ServiceStatus{MobsLoaded: snap.SourceCount, LastReload: lastReload, LastError: s.lastError}
}
