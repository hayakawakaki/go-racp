package app

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/mob/domain"
	"github.com/hayakawakaki/go-racp/internal/platform/refdata"
)

type LoaderFunc func() (*domain.Snapshot, error)

type Service struct {
	refdata.ReloadGuard[domain.Snapshot]
}

func NewService(loader LoaderFunc) *Service {
	return &Service{ReloadGuard: refdata.ReloadGuard[domain.Snapshot]{Loader: loader}}
}

func NewServiceWithSnapshot(snap *domain.Snapshot, loader LoaderFunc) *Service {
	service := NewService(loader)
	service.Store(snap)

	return service
}

func (s *Service) Snapshot() *domain.Snapshot {
	snap := s.Load()
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

	out := make([]*domain.Mob, 0, 32)
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

	return entries
}

func (s *Service) Status() ServiceStatus {
	snap := s.Snapshot()
	lastReload := ""
	if last := s.LastReload(); !last.IsZero() {
		lastReload = last.Format(time.RFC3339)
	}

	return ServiceStatus{MobsLoaded: snap.SourceCount, LastReload: lastReload, LastError: s.LastError()}
}
