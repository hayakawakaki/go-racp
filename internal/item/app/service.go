package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hayakawakaki/go-racp/internal/item/domain"
)

type LoaderFunc func() (*domain.Snapshot, error)

type ListQuery struct {
	Query   string
	Type    domain.ItemType
	Page    int
	PerPage int
}

//nolint:govet // fieldalignment: 8-byte gain on a singleton
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

func (s *Service) Get(_ context.Context, id int) (*domain.Item, error) {
	snap := s.Snapshot()
	if snap.SourceCount == 0 {
		return nil, domain.ErrEmptySnapshot
	}
	item, ok := snap.ByID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}

	return item, nil
}

func (s *Service) List(_ context.Context, query ListQuery) (ItemPage, error) {
	snap := s.Snapshot()
	if query.PerPage <= 0 {
		query.PerPage = 20
	}
	if query.Page <= 0 {
		query.Page = 1
	}

	filtered := filterItems(snap.Sorted, query)
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

	return ItemPage{
		Items:      filtered[start:end],
		Total:      total,
		Page:       query.Page,
		PerPage:    query.PerPage,
		TotalPages: totalPages,
	}, nil
}

func filterItems(items []*domain.Item, query ListQuery) []*domain.Item {
	if query.Query == "" && query.Type == domain.ItemTypeUnknown {
		return items
	}
	needle := strings.ToLower(strings.TrimSpace(query.Query))
	var asID int
	if needle != "" {
		if parsed, err := strconv.Atoi(needle); err == nil {
			asID = parsed
		}
	}

	out := make([]*domain.Item, 0, len(items))
	for _, item := range items {
		if query.Type != domain.ItemTypeUnknown && item.Type != query.Type {
			continue
		}
		if needle != "" && !matchesQuery(item, needle, asID) {
			continue
		}
		out = append(out, item)
	}

	return out
}

func matchesQuery(item *domain.Item, needle string, asID int) bool {
	if asID > 0 && item.ID == asID {
		return true
	}
	if strings.Contains(strings.ToLower(item.AegisName), needle) {
		return true
	}
	if strings.Contains(strings.ToLower(item.ClientName), needle) {
		return true
	}

	return false
}

func (s *Service) Reload(_ context.Context) error {
	if !s.reloadMu.TryLock() {
		return domain.ErrReloadConflict
	}
	defer s.reloadMu.Unlock()

	if s.loader == nil {
		return fmt.Errorf("app.Service.Reload: %w", domain.ErrParseFailed)
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

	return ServiceStatus{ItemsLoaded: snap.SourceCount, LastReload: lastReload, LastError: s.lastError}
}
