package app

import (
	"context"
	"sort"

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

	page := filtered[start:end]
	out := make([]domain.Vendor, len(page))
	copy(out, page)

	return domain.Page{
		Vendors:    out,
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

		sortByItemPrice(out, q.ItemID, q.Type)
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

func sortByItemPrice(vendors []domain.Vendor, itemID int, t domain.VendorType) {
	sort.SliceStable(vendors, func(i, j int) bool {
		pi := vendorItemPrice(vendors[i], itemID)
		pj := vendorItemPrice(vendors[j], itemID)
		if t == domain.VendorTypeBuying {
			return pi > pj
		}
		return pi < pj
	})
}

func vendorItemPrice(v domain.Vendor, itemID int) int {
	for _, item := range v.Items {
		if item.ItemID == itemID {
			return item.Price
		}
	}
	return 0
}
