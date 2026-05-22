package transport

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/a-h/templ"
	itemdomain "github.com/hayakawakaki/go-racp/internal/features/item/domain"
	"github.com/hayakawakaki/go-racp/internal/features/stall/domain"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
	_ "github.com/hayakawakaki/go-racp/themes/default/platform/httpx"
)

type stubTheme struct{}

func (stubTheme) StallListPage(layout httpx.Layout, state ListState) templ.Component {
	return StallListPage(layout, state)
}

func (stubTheme) StallListContent(state ListState) templ.Component {
	return StallListContent(state)
}

func (stubTheme) StallLoadingPage(layout httpx.Layout, refreshURL string) templ.Component {
	return StallLoadingPage(layout, refreshURL)
}

func (stubTheme) StallLoadingContent(refreshURL string) templ.Component {
	return StallLoadingContent(refreshURL)
}

func (stubTheme) StallVendingBox(state StallState) templ.Component {
	return StallVendingBox(state)
}

type fakeService struct {
	listErr    error
	getErr     error
	listResult domain.Page
	getResult  domain.Vendor
	listQuery  domain.ListQuery
	lastGetKey domain.VendorKey
	listCalls  int
	getCalls   int
}

func (f *fakeService) List(_ context.Context, q domain.ListQuery) (domain.Page, error) {
	f.listCalls++
	f.listQuery = q

	return f.listResult, f.listErr
}

func (f *fakeService) Get(_ context.Context, key domain.VendorKey) (domain.Vendor, error) {
	f.getCalls++
	f.lastGetKey = key

	return f.getResult, f.getErr
}

type fakeItemLookup struct {
	items map[int]*itemdomain.Item
	err   error
}

func (f *fakeItemLookup) Get(_ context.Context, id int) (*itemdomain.Item, error) {
	if f.err != nil {
		return nil, f.err
	}
	item, ok := f.items[id]
	if !ok {
		return nil, itemdomain.ErrNotFound
	}

	return item, nil
}

func newTestHandler(svc *fakeService, lookup *fakeItemLookup) *Handler {
	return NewHandler(svc, HandlerConfig{ItemLookup: lookup, Theme: stubTheme{}})
}

func TestHandler_ShowList_HappyPath(t *testing.T) {
	t.Parallel()
	svc := &fakeService{listResult: domain.Page{
		Vendors: []domain.Vendor{{ID: 1, Type: domain.VendorTypeSelling, StallName: "stall-1", SellerName: "testuser"}},
		Total:   1, Page: 1, PerPage: 20, TotalPages: 1,
	}}
	h := newTestHandler(svc, &fakeItemLookup{})

	r := httptest.NewRequest(http.MethodGet, "/vendors?type=selling&item=501&page=2", http.NoBody)
	w := httptest.NewRecorder()
	h.showList(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if svc.listQuery.Type != domain.VendorTypeSelling {
		t.Errorf("Type = %v, want selling", svc.listQuery.Type)
	}
	if svc.listQuery.ItemID != 501 {
		t.Errorf("ItemID = %d, want 501", svc.listQuery.ItemID)
	}
	if svc.listQuery.Page != 2 {
		t.Errorf("Page = %d, want 2", svc.listQuery.Page)
	}
	if !strings.Contains(w.Body.String(), "stall-1") {
		t.Errorf("body missing stall name; got: %s", w.Body.String())
	}
}

func TestHandler_ShowList_SnapshotNotReadyRendersLoading(t *testing.T) {
	t.Parallel()
	svc := &fakeService{listErr: domain.ErrSnapshotNotReady}
	h := newTestHandler(svc, &fakeItemLookup{})

	r := httptest.NewRequest(http.MethodGet, "/vendors", http.NoBody)
	w := httptest.NewRecorder()
	h.showList(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(strings.ToLower(body), "loading") {
		t.Errorf("body missing loading indicator: %s", body)
	}
	if !strings.Contains(body, "<html") && !strings.Contains(body, "<!doctype") {
		t.Errorf("non-HTMX request should return a full page; got fragment: %s", body[:min(len(body), 200)])
	}
}

func TestHandler_ShowList_SnapshotNotReadyHTMXReturnsFragment(t *testing.T) {
	t.Parallel()
	svc := &fakeService{listErr: domain.ErrSnapshotNotReady}
	h := newTestHandler(svc, &fakeItemLookup{})

	r := httptest.NewRequest(http.MethodGet, "/vendors", http.NoBody)
	r.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	h.showList(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if strings.Contains(body, "<html") || strings.Contains(body, "<!doctype") {
		t.Errorf("HTMX request should return a fragment, got full page: %s", body[:min(len(body), 200)])
	}
	if !strings.Contains(strings.ToLower(body), "loading") {
		t.Errorf("body missing loading indicator: %s", body)
	}
}

func TestHandler_ShowList_SnapshotNotReadyPreservesFiltersInRefreshURL(t *testing.T) {
	t.Parallel()
	svc := &fakeService{listErr: domain.ErrSnapshotNotReady}
	h := newTestHandler(svc, &fakeItemLookup{})

	r := httptest.NewRequest(http.MethodGet, "/vendors?type=selling&item=501&page=2", http.NoBody)
	w := httptest.NewRecorder()
	h.showList(w, r)

	body := w.Body.String()
	if !strings.Contains(body, "type=selling") || !strings.Contains(body, "item=501") || !strings.Contains(body, "page=2") {
		t.Errorf("loading body should embed current query in refresh URL; got: %s", body)
	}
}

func TestHandler_ShowList_ServiceErrorReturns500(t *testing.T) {
	t.Parallel()
	svc := &fakeService{listErr: errors.New("boom")}
	h := newTestHandler(svc, &fakeItemLookup{})

	r := httptest.NewRequest(http.MethodGet, "/vendors", http.NoBody)
	w := httptest.NewRecorder()
	h.showList(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestHandler_ShowStallItems_HappyPath(t *testing.T) {
	t.Parallel()
	svc := &fakeService{getResult: domain.Vendor{
		ID: 5, Type: domain.VendorTypeSelling, StallName: "stall-x", SellerName: "testuser",
		Items: []domain.VendorItem{{ItemID: 501, Amount: 2, Price: 100}},
	}}
	lookup := &fakeItemLookup{items: map[int]*itemdomain.Item{
		501: {ID: 501, AegisName: "Red_Potion", ClientName: "Red Potion"},
	}}
	h := newTestHandler(svc, lookup)

	r := httptest.NewRequest(http.MethodGet, "/vendors/selling/5/items", http.NoBody)
	r.SetPathValue("type", "selling")
	r.SetPathValue("id", "5")
	w := httptest.NewRecorder()
	h.showStallItems(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if svc.lastGetKey != (domain.VendorKey{Type: domain.VendorTypeSelling, ID: 5}) {
		t.Errorf("lastGetKey = %+v", svc.lastGetKey)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Red Potion") {
		t.Errorf("body missing resolved item name: %s", body)
	}
}

func TestHandler_ShowStallItems_InvalidTypeReturns404(t *testing.T) {
	t.Parallel()
	svc := &fakeService{}
	h := newTestHandler(svc, &fakeItemLookup{})

	r := httptest.NewRequest(http.MethodGet, "/vendors/bogus/1/items", http.NoBody)
	r.SetPathValue("type", "bogus")
	r.SetPathValue("id", "1")
	w := httptest.NewRecorder()
	h.showStallItems(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
	if svc.getCalls != 0 {
		t.Errorf("service was called for invalid type")
	}
}

func TestHandler_ShowStallItems_NotFoundReturns404(t *testing.T) {
	t.Parallel()
	svc := &fakeService{getErr: domain.ErrVendorNotFound}
	h := newTestHandler(svc, &fakeItemLookup{})

	r := httptest.NewRequest(http.MethodGet, "/vendors/selling/9999/items", http.NoBody)
	r.SetPathValue("type", "selling")
	r.SetPathValue("id", "9999")
	w := httptest.NewRecorder()
	h.showStallItems(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestHandler_ShowStallItems_SnapshotNotReadyReturns503(t *testing.T) {
	t.Parallel()
	svc := &fakeService{getErr: domain.ErrSnapshotNotReady}
	h := newTestHandler(svc, &fakeItemLookup{})

	r := httptest.NewRequest(http.MethodGet, "/vendors/selling/1/items", http.NoBody)
	r.SetPathValue("type", "selling")
	r.SetPathValue("id", "1")
	w := httptest.NewRecorder()
	h.showStallItems(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}
}
