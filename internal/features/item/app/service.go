package app

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/item/domain"
	"github.com/hayakawakaki/go-racp/internal/refdata"
)

type LoaderFunc func() (*domain.Snapshot, error)

type ListQuery struct {
	Query   string
	Type    domain.ItemType
	Page    int
	PerPage int
}

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

func (s *Service) LookupByAegis(aegis string) *domain.Item {
	if aegis == "" {
		return nil
	}
	snap := s.Snapshot()
	if snap.SourceCount == 0 {
		return nil
	}

	return snap.ByName[aegis]
}

func (s *Service) List(_ context.Context, query ListQuery) (ItemPage, error) {
	snap := s.Snapshot()
	if query.PerPage <= 0 {
		query.PerPage = DefaultPerPage
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
	if strings.Contains(item.AegisNameLower, needle) {
		return true
	}
	if strings.Contains(item.ClientNameLower, needle) {
		return true
	}

	return false
}

func (s *Service) Status() ServiceStatus {
	snap := s.Snapshot()
	lastReload := ""
	if last := s.LastReload(); !last.IsZero() {
		lastReload = last.Format(time.RFC3339)
	}

	return ServiceStatus{ItemsLoaded: snap.SourceCount, LastReload: lastReload, LastError: s.LastError()}
}
