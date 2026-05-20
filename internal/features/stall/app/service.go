package app

import (
	"context"

	"github.com/hayakawakaki/go-racp/internal/features/stall/domain"
)

const DefaultPerPage = 20

type SnapshotSource interface {
	Snapshot() *domain.Snapshot
}

type Service struct {
	source SnapshotSource
}

func NewService(source SnapshotSource) *Service {
	return &Service{source: source}
}

func (s *Service) List(_ context.Context, q domain.ListQuery) (domain.Page, error) {
	snap := s.source.Snapshot()
	if snap == nil {
		return domain.Page{}, domain.ErrSnapshotNotReady
	}
	if q.PerPage <= 0 {
		q.PerPage = DefaultPerPage
	}
	if q.Page <= 0 {
		q.Page = 1
	}

	filtered := filterVendors(snap, q)
	total := len(filtered)
	totalPages := (total + q.PerPage - 1) / q.PerPage
	start := (q.Page - 1) * q.PerPage
	end := start + q.PerPage
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	return domain.Page{
		Vendors:    filtered[start:end],
		Total:      total,
		Page:       q.Page,
		PerPage:    q.PerPage,
		TotalPages: totalPages,
	}, nil
}

func (s *Service) Get(_ context.Context, key domain.VendorKey) (domain.Vendor, error) {
	snap := s.source.Snapshot()
	if snap == nil {
		return domain.Vendor{}, domain.ErrSnapshotNotReady
	}
	v, ok := snap.ByKey[key]
	if !ok {
		return domain.Vendor{}, domain.ErrVendorNotFound
	}

	return *v, nil
}

func filterVendors(snap *domain.Snapshot, q domain.ListQuery) []domain.Vendor {
	if q.ItemID > 0 {
		keys := snap.ByItem[q.ItemID]
		out := make([]domain.Vendor, 0, len(keys))
		for _, key := range keys {
			v := snap.ByKey[key]
			if v == nil {
				continue
			}
			if q.Type != domain.VendorTypeUnknown && v.Type != q.Type {
				continue
			}
			out = append(out, *v)
		}

		return out
	}

	if q.Type == domain.VendorTypeUnknown {
		return snap.Vendors
	}

	out := make([]domain.Vendor, 0, len(snap.Vendors))
	for _, v := range snap.Vendors {
		if v.Type != q.Type {
			continue
		}
		out = append(out, v)
	}

	return out
}
