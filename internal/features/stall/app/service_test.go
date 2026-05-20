package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/stall/domain"
)

type fakeSnapshotSource struct {
	snap *domain.Snapshot
}

func (f *fakeSnapshotSource) Snapshot() *domain.Snapshot { return f.snap }

func newSnapshot(vendors ...domain.Vendor) *domain.Snapshot {
	return buildSnapshot(vendors)
}

func TestService_List_FiltersByType(t *testing.T) {
	t.Parallel()
	snap := newSnapshot(
		domain.Vendor{ID: 1, Type: domain.VendorTypeSelling},
		domain.Vendor{ID: 2, Type: domain.VendorTypeBuying},
		domain.Vendor{ID: 3, Type: domain.VendorTypeSelling},
	)
	svc := NewService(&fakeSnapshotSource{snap: snap})

	page, err := svc.List(context.Background(), domain.ListQuery{Type: domain.VendorTypeSelling, Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if page.Total != 2 || len(page.Vendors) != 2 {
		t.Errorf("got total=%d len=%d, want 2/2", page.Total, len(page.Vendors))
	}
	for _, v := range page.Vendors {
		if v.Type != domain.VendorTypeSelling {
			t.Errorf("non-selling vendor in result: %+v", v)
		}
	}
}

func TestService_List_FiltersByItemID(t *testing.T) {
	t.Parallel()
	snap := newSnapshot(
		domain.Vendor{ID: 1, Type: domain.VendorTypeSelling, Items: []domain.VendorItem{{ItemID: 501}}},
		domain.Vendor{ID: 2, Type: domain.VendorTypeBuying, Items: []domain.VendorItem{{ItemID: 501}}},
		domain.Vendor{ID: 3, Type: domain.VendorTypeSelling, Items: []domain.VendorItem{{ItemID: 502}}},
	)
	svc := NewService(&fakeSnapshotSource{snap: snap})

	page, err := svc.List(context.Background(), domain.ListQuery{ItemID: 501, Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if page.Total != 2 {
		t.Errorf("Total = %d, want 2", page.Total)
	}
}

func TestService_List_Pagination(t *testing.T) {
	t.Parallel()
	vendors := make([]domain.Vendor, 25)
	for i := range vendors {
		vendors[i] = domain.Vendor{ID: i + 1, Type: domain.VendorTypeSelling}
	}
	snap := newSnapshot(vendors...)
	svc := NewService(&fakeSnapshotSource{snap: snap})

	page, err := svc.List(context.Background(), domain.ListQuery{Page: 2, PerPage: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if page.Total != 25 || page.TotalPages != 3 {
		t.Errorf("total=%d pages=%d, want 25/3", page.Total, page.TotalPages)
	}
	if len(page.Vendors) != 10 {
		t.Errorf("len(page.Vendors) = %d, want 10", len(page.Vendors))
	}
}

func TestService_List_SnapshotNotReady(t *testing.T) {
	t.Parallel()
	svc := NewService(&fakeSnapshotSource{snap: nil})

	_, err := svc.List(context.Background(), domain.ListQuery{Page: 1, PerPage: 10})
	if !errors.Is(err, domain.ErrSnapshotNotReady) {
		t.Errorf("err = %v, want ErrSnapshotNotReady", err)
	}
}

func TestService_Get_HappyPath(t *testing.T) {
	t.Parallel()
	snap := newSnapshot(domain.Vendor{ID: 42, Type: domain.VendorTypeSelling, StallName: "x"})
	snap.LoadedAt = time.Now()
	svc := NewService(&fakeSnapshotSource{snap: snap})

	v, err := svc.Get(context.Background(), domain.VendorKey{Type: domain.VendorTypeSelling, ID: 42})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if v.ID != 42 {
		t.Errorf("v.ID = %d, want 42", v.ID)
	}
}

func TestService_Get_NotFound(t *testing.T) {
	t.Parallel()
	snap := newSnapshot()
	svc := NewService(&fakeSnapshotSource{snap: snap})

	_, err := svc.Get(context.Background(), domain.VendorKey{Type: domain.VendorTypeSelling, ID: 1})
	if !errors.Is(err, domain.ErrVendorNotFound) {
		t.Errorf("err = %v, want ErrVendorNotFound", err)
	}
}

func TestService_Get_SnapshotNotReady(t *testing.T) {
	t.Parallel()
	svc := NewService(&fakeSnapshotSource{snap: nil})

	_, err := svc.Get(context.Background(), domain.VendorKey{Type: domain.VendorTypeSelling, ID: 1})
	if !errors.Is(err, domain.ErrSnapshotNotReady) {
		t.Errorf("err = %v, want ErrSnapshotNotReady", err)
	}
}
