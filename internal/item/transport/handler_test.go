package transport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	itemapp "github.com/hayakawakaki/go-racp/internal/item/app"
	"github.com/hayakawakaki/go-racp/internal/item/domain"
	mobdomain "github.com/hayakawakaki/go-racp/internal/mob/domain"
	"github.com/hayakawakaki/go-racp/internal/refdata"
	"github.com/hayakawakaki/go-racp/server/config"
)

//nolint:govet // fine for test
type fakeItemService struct {
	itemsByID   map[int]*domain.Item
	getErr      error
	listErr     error
	reloadErr   error
	listResp    itemapp.ItemPage
	gotListArg  itemapp.ListQuery
	statusResp  itemapp.ServiceStatus
	reloadCalls int
}

func newFakeItemService() *fakeItemService {
	return &fakeItemService{itemsByID: map[int]*domain.Item{}}
}

func (s *fakeItemService) Get(_ context.Context, id int) (*domain.Item, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	item, ok := s.itemsByID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}

	return item, nil
}

func (s *fakeItemService) List(_ context.Context, query itemapp.ListQuery) (itemapp.ItemPage, error) {
	s.gotListArg = query
	if s.listErr != nil {
		return itemapp.ItemPage{}, s.listErr
	}

	return s.listResp, nil
}

func (s *fakeItemService) Reload(_ context.Context) error {
	s.reloadCalls++

	return s.reloadErr
}

func (s *fakeItemService) Status() itemapp.ServiceStatus { return s.statusResp }

type fakeDropLookup struct {
	byAegis map[string][]mobdomain.DropOf
	gotArg  string
}

func (l *fakeDropLookup) WhoDrops(itemAegis string) []mobdomain.DropOf {
	l.gotArg = itemAegis

	return l.byAegis[itemAegis]
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestHandler(svc itemService) *Handler {
	return NewHandler(svc, HandlerConfig{
		Logger:  discardLogger(),
		General: config.GeneralConfig{ServerName: "Test", Timezone: "UTC"},
	})
}

func newTestHandlerWithDropLookup(svc itemService, lookup dropLookup) *Handler {
	return NewHandler(svc, HandlerConfig{
		Logger:     discardLogger(),
		DropLookup: lookup,
		General:    config.GeneralConfig{ServerName: "Test", Timezone: "UTC"},
	})
}

func TestHandler_ShowList_EmptySnapshotReturnsEmptyDatabasePage(t *testing.T) {
	t.Parallel()

	svc := newFakeItemService()
	svc.listErr = domain.ErrEmptySnapshot
	h := newTestHandler(svc)

	rr := httptest.NewRecorder()
	h.showList(rr, httptest.NewRequest(http.MethodGet, "/items", http.NoBody))

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
	if !strings.Contains(rr.Body.String(), "No items loaded") {
		t.Errorf("body missing empty-database marker:\n%s", rr.Body.String())
	}
}

func TestHandler_ShowList_RendersResults(t *testing.T) {
	t.Parallel()

	item := &domain.Item{ID: 501, AegisName: "Red_Potion", ClientName: "Red Potion", Type: domain.ItemTypeHealing, Image: "red_potion"}
	svc := newFakeItemService()
	svc.listResp = itemapp.ItemPage{
		Items: []*domain.Item{item}, Total: 1, Page: 1, PerPage: itemapp.DefaultPerPage, TotalPages: 1,
	}
	h := newTestHandler(svc)

	rr := httptest.NewRecorder()
	h.showList(rr, httptest.NewRequest(http.MethodGet, "/items", http.NoBody))

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Red Potion") {
		t.Errorf("body missing item ClientName:\n%s", body)
	}
}

func TestHandler_ShowList_ServiceErrorReturns500(t *testing.T) {
	t.Parallel()

	svc := newFakeItemService()
	svc.listErr = errors.New("boom")
	h := newTestHandler(svc)

	rr := httptest.NewRecorder()
	h.showList(rr, httptest.NewRequest(http.MethodGet, "/items", http.NoBody))

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
}

func TestHandler_ShowList_ParsesQueryParameters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		url         string
		wantQuery   string
		wantPage    int
		wantPerPage int
		wantType    domain.ItemType
	}{
		{
			name:        "defaults when no params",
			url:         "/items",
			wantPage:    1,
			wantQuery:   "",
			wantType:    domain.ItemTypeUnknown,
			wantPerPage: itemapp.DefaultPerPage,
		},
		{
			name:        "page param",
			url:         "/items?page=3",
			wantPage:    3,
			wantPerPage: itemapp.DefaultPerPage,
		},
		{
			name:        "negative page falls back",
			url:         "/items?page=-5",
			wantPage:    1,
			wantPerPage: itemapp.DefaultPerPage,
		},
		{
			name:        "non-numeric page falls back",
			url:         "/items?page=abc",
			wantPage:    1,
			wantPerPage: itemapp.DefaultPerPage,
		},
		{
			name:        "query and type",
			url:         "/items?q=red&type=Healing",
			wantPage:    1,
			wantQuery:   "red",
			wantType:    domain.ItemTypeHealing,
			wantPerPage: itemapp.DefaultPerPage,
		},
		{
			name:        "unknown type is ignored",
			url:         "/items?type=Mystery",
			wantPage:    1,
			wantType:    domain.ItemTypeUnknown,
			wantPerPage: itemapp.DefaultPerPage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := newFakeItemService()
			h := newTestHandler(svc)

			rr := httptest.NewRecorder()
			h.showList(rr, httptest.NewRequest(http.MethodGet, tt.url, http.NoBody))

			if svc.gotListArg.Page != tt.wantPage {
				t.Errorf("Page = %d, want %d", svc.gotListArg.Page, tt.wantPage)
			}
			if svc.gotListArg.Query != tt.wantQuery {
				t.Errorf("Query = %q, want %q", svc.gotListArg.Query, tt.wantQuery)
			}
			if svc.gotListArg.Type != tt.wantType {
				t.Errorf("Type = %v, want %v", svc.gotListArg.Type, tt.wantType)
			}
			if svc.gotListArg.PerPage != tt.wantPerPage {
				t.Errorf("PerPage = %d, want %d", svc.gotListArg.PerPage, tt.wantPerPage)
			}
		})
	}
}

func TestHandler_ShowDetail_NotFound(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		id   string
	}{
		{name: "non-numeric id", id: "abc"},
		{name: "zero id", id: "0"},
		{name: "negative id", id: "-5"},
		{name: "missing item", id: "9999"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := newFakeItemService()
			h := newTestHandler(svc)

			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/items/"+tt.id, http.NoBody)
			req.SetPathValue("id", tt.id)
			h.showDetail(rr, req)

			if rr.Code != http.StatusNotFound {
				t.Errorf("status = %d, want 404", rr.Code)
			}
			if !strings.Contains(rr.Body.String(), "Item not found") {
				t.Errorf("body missing not-found marker:\n%s", rr.Body.String())
			}
		})
	}
}

func TestHandler_ShowDetail_EmptySnapshotRendersNotFound(t *testing.T) {
	t.Parallel()

	svc := newFakeItemService()
	svc.getErr = domain.ErrEmptySnapshot
	h := newTestHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/items/501", http.NoBody)
	req.SetPathValue("id", "501")
	h.showDetail(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

func TestHandler_ShowDetail_ServiceErrorReturns500(t *testing.T) {
	t.Parallel()

	svc := newFakeItemService()
	svc.getErr = errors.New("db down")
	h := newTestHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/items/501", http.NoBody)
	req.SetPathValue("id", "501")
	h.showDetail(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
}

func TestHandler_ShowDetail_RendersItem(t *testing.T) {
	t.Parallel()

	svc := newFakeItemService()
	svc.itemsByID[501] = &domain.Item{
		ID:          501,
		AegisName:   "Red_Potion",
		Name:        "Red Potion",
		ClientName:  "Red Potion",
		Image:       "red_potion",
		Type:        domain.ItemTypeHealing,
		Description: []string{"A bottle of potion."},
		Weight:      7.0,
		Buy:         10,
	}
	h := newTestHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/items/501", http.NoBody)
	req.SetPathValue("id", "501")
	h.showDetail(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Red Potion") {
		t.Errorf("body missing item name:\n%s", body)
	}
}

func TestHandler_ShowDetail_NoDropLookupSkipsDroppedBy(t *testing.T) {
	t.Parallel()

	svc := newFakeItemService()
	svc.itemsByID[501] = &domain.Item{ID: 501, AegisName: "Red_Potion", Name: "Red Potion", ClientName: "Red Potion"}
	h := newTestHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/items/501", http.NoBody)
	req.SetPathValue("id", "501")
	h.showDetail(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if strings.Contains(rr.Body.String(), "Dropped By") {
		t.Errorf("body contains Dropped By section without lookup configured")
	}
}

func TestHandler_ShowDetail_DropLookupRendersDroppedBy(t *testing.T) {
	t.Parallel()

	svc := newFakeItemService()
	svc.itemsByID[501] = &domain.Item{ID: 501, AegisName: "Red_Potion", Name: "Red Potion", ClientName: "Red Potion"}
	lookup := &fakeDropLookup{byAegis: map[string][]mobdomain.DropOf{
		"Red_Potion": {{MobID: 1002, MobName: "Poring", Rate: 1000}},
	}}
	h := newTestHandlerWithDropLookup(svc, lookup)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/items/501", http.NoBody)
	req.SetPathValue("id", "501")
	h.showDetail(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if lookup.gotArg != "Red_Potion" {
		t.Errorf("WhoDrops arg = %q, want Red_Potion", lookup.gotArg)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Dropped By") {
		t.Errorf("body missing Dropped By section:\n%s", body)
	}
	if !strings.Contains(body, "Poring") {
		t.Errorf("body missing mob name:\n%s", body)
	}
}

func TestHandler_ShowDetail_DropLookupEmptyResultOmitsSection(t *testing.T) {
	t.Parallel()

	svc := newFakeItemService()
	svc.itemsByID[501] = &domain.Item{ID: 501, AegisName: "Red_Potion", Name: "Red Potion", ClientName: "Red Potion"}
	lookup := &fakeDropLookup{byAegis: map[string][]mobdomain.DropOf{}}
	h := newTestHandlerWithDropLookup(svc, lookup)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/items/501", http.NoBody)
	req.SetPathValue("id", "501")
	h.showDetail(rr, req)

	if strings.Contains(rr.Body.String(), "Dropped By") {
		t.Errorf("body contains Dropped By section when lookup returns no results")
	}
}

func TestHandler_APIDetail_InvalidIDReturns404(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		id   string
	}{
		{name: "non-numeric", id: "abc"},
		{name: "zero", id: "0"},
		{name: "negative", id: "-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := newFakeItemService()
			h := newTestHandler(svc)

			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/items/"+tt.id, http.NoBody)
			req.SetPathValue("id", tt.id)
			h.apiDetail(rr, req)

			if rr.Code != http.StatusNotFound {
				t.Errorf("status = %d, want 404", rr.Code)
			}
			var body apiError
			if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if body.Error != "item not found" {
				t.Errorf("error = %q, want %q", body.Error, "item not found")
			}
			if body.ID != 0 {
				t.Errorf("ID = %d, want 0 for invalid input", body.ID)
			}
		})
	}
}

func TestHandler_APIDetail_NotFoundPreservesID(t *testing.T) {
	t.Parallel()

	svc := newFakeItemService()
	h := newTestHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/items/9999", http.NoBody)
	req.SetPathValue("id", "9999")
	h.apiDetail(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
	var body apiError
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.ID != 9999 {
		t.Errorf("ID = %d, want 9999 (echoed for valid numeric input)", body.ID)
	}
}

func TestHandler_APIDetail_EmptySnapshotReturns404(t *testing.T) {
	t.Parallel()

	svc := newFakeItemService()
	svc.getErr = domain.ErrEmptySnapshot
	h := newTestHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/items/501", http.NoBody)
	req.SetPathValue("id", "501")
	h.apiDetail(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

func TestHandler_APIDetail_ServiceErrorReturns500(t *testing.T) {
	t.Parallel()

	svc := newFakeItemService()
	svc.getErr = errors.New("db down")
	h := newTestHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/items/501", http.NoBody)
	req.SetPathValue("id", "501")
	h.apiDetail(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
}

func TestHandler_APIDetail_Success(t *testing.T) {
	t.Parallel()

	svc := newFakeItemService()
	svc.itemsByID[501] = &domain.Item{
		ID:         501,
		AegisName:  "Red_Potion",
		Name:       "Red Potion",
		ClientName: "Red Potion",
		Image:      "red_potion",
		Type:       domain.ItemTypeHealing,
		Buy:        10,
		Sell:       5,
		Weight:     7.0,
	}
	h := newTestHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/items/501", http.NoBody)
	req.SetPathValue("id", "501")
	h.apiDetail(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}

	var got itemapp.ItemDTO
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID != 501 || got.AegisName != "Red_Potion" || got.Type != "Healing" {
		t.Errorf("body = %+v", got)
	}
}

func TestHandler_DoReload_Success(t *testing.T) {
	t.Parallel()

	svc := newFakeItemService()
	svc.statusResp = itemapp.ServiceStatus{ItemsLoaded: 7, LastReload: time.Now().Format(time.RFC3339)}
	h := newTestHandler(svc)

	rr := httptest.NewRecorder()
	h.doReload(rr, httptest.NewRequest(http.MethodPost, "/admin/items/reload", http.NoBody))

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if svc.reloadCalls != 1 {
		t.Errorf("reload calls = %d, want 1", svc.reloadCalls)
	}
}

func TestHandler_DoReload_ConflictReturns409(t *testing.T) {
	t.Parallel()

	svc := newFakeItemService()
	svc.reloadErr = refdata.ErrReloadConflict
	h := newTestHandler(svc)

	rr := httptest.NewRecorder()
	h.doReload(rr, httptest.NewRequest(http.MethodPost, "/admin/items/reload", http.NoBody))

	if rr.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rr.Code)
	}
}

func TestHandler_DoReload_FailureReturns500(t *testing.T) {
	t.Parallel()

	svc := newFakeItemService()
	svc.reloadErr = errors.New("disk full")
	h := newTestHandler(svc)

	rr := httptest.NewRecorder()
	h.doReload(rr, httptest.NewRequest(http.MethodPost, "/admin/items/reload", http.NoBody))

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
}

func TestNewHandler_NilLoggerFallsBackToDefault(t *testing.T) {
	t.Parallel()

	h := NewHandler(newFakeItemService(), HandlerConfig{})
	if h.logger == nil {
		t.Errorf("logger is nil, want fallback slog.Default")
	}
}

func TestDroppedByAlpineState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		want  string
		count int
	}{
		{name: "zero entries clamps to one page", count: 0, want: "{ page: 1, perPage: 10, totalPages: 1 }"},
		{name: "fits in one page", count: 7, want: "{ page: 1, perPage: 10, totalPages: 1 }"},
		{name: "exactly one page", count: 10, want: "{ page: 1, perPage: 10, totalPages: 1 }"},
		{name: "rolls over to two pages", count: 11, want: "{ page: 1, perPage: 10, totalPages: 2 }"},
		{name: "many pages", count: 35, want: "{ page: 1, perPage: 10, totalPages: 4 }"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := droppedByAlpineState(tt.count); got != tt.want {
				t.Errorf("droppedByAlpineState(%d) = %q, want %q", tt.count, got, tt.want)
			}
		})
	}
}

func TestYesNo(t *testing.T) {
	t.Parallel()

	if got := yesNo(true); got != "Yes" {
		t.Errorf("yesNo(true) = %q, want Yes", got)
	}
	if got := yesNo(false); got != "No" {
		t.Errorf("yesNo(false) = %q, want No", got)
	}
}

func TestLocationLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
		in   domain.ItemType
	}{
		{name: "weapon", in: domain.ItemTypeWeapon, want: "Equip Location"},
		{name: "armor", in: domain.ItemTypeArmor, want: "Equip Location"},
		{name: "card", in: domain.ItemTypeCard, want: "Slot Location"},
		{name: "healing has no label", in: domain.ItemTypeHealing, want: ""},
		{name: "ammo has no label", in: domain.ItemTypeAmmo, want: ""},
		{name: "unknown has no label", in: domain.ItemTypeUnknown, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := locationLabel(tt.in); got != tt.want {
				t.Errorf("locationLabel(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestBuildStats_HealingItemUsesBaseRowsOnly(t *testing.T) {
	t.Parallel()

	item := &domain.Item{Type: domain.ItemTypeHealing, Weight: 7, Buy: 10, Sell: 5}
	rows := buildStats(item)
	labels := rowLabels(rows)

	wantLabels := []string{"Type", "Weight", "Buy", "Sell"}
	if !slices.Equal(labels, wantLabels) {
		t.Errorf("labels = %v, want %v", labels, wantLabels)
	}
}

func TestBuildStats_WeaponAppendsCombatRows(t *testing.T) {
	t.Parallel()

	var locations domain.LocationSet
	locations.Set(domain.LocationRightHand)
	item := &domain.Item{
		Type:        domain.ItemTypeWeapon,
		Weight:      5.5,
		Buy:         100,
		Sell:        50,
		WeaponLevel: 2,
		Attack:      25,
		Range:       1,
		Slots:       3,
		Refineable:  true,
		Locations:   locations,
		SubType:     "1hSword",
	}
	rows := buildStats(item)
	labels := rowLabels(rows)

	for _, want := range []string{"Type", "Weight", "Buy", "Sell", "Weapon Level", "Attack", "Range", "Slots", "Refineable", "Equip Location", "Subtype"} {
		if !slices.Contains(labels, want) {
			t.Errorf("labels = %v, missing %q", labels, want)
		}
	}
}

func TestBuildStats_ArmorAppendsDefensiveRows(t *testing.T) {
	t.Parallel()

	var locations domain.LocationSet
	locations.Set(domain.LocationArmor)
	item := &domain.Item{
		Type:       domain.ItemTypeArmor,
		ArmorLevel: 1,
		Defense:    15,
		Slots:      1,
		Refineable: true,
		Locations:  locations,
	}
	rows := buildStats(item)
	labels := rowLabels(rows)

	for _, want := range []string{"Armor Level", "Defense", "Slots", "Refineable", "Equip Location"} {
		if !slices.Contains(labels, want) {
			t.Errorf("labels = %v, missing %q", labels, want)
		}
	}
}

func TestBuildStats_NoLocationsOmitsLocationRow(t *testing.T) {
	t.Parallel()

	item := &domain.Item{Type: domain.ItemTypeWeapon, WeaponLevel: 1}
	rows := buildStats(item)
	labels := rowLabels(rows)
	if slices.Contains(labels, "Equip Location") {
		t.Errorf("labels = %v, should not contain Equip Location when LocationSet is empty", labels)
	}
}

func TestBuildStats_EmptySubTypeIsOmitted(t *testing.T) {
	t.Parallel()

	item := &domain.Item{Type: domain.ItemTypeHealing}
	rows := buildStats(item)
	if slices.Contains(rowLabels(rows), "Subtype") {
		t.Errorf("Subtype row appeared when SubType is empty")
	}
}

func rowLabels(rows []labeledRow) []string {
	labels := make([]string, len(rows))
	for index, row := range rows {
		labels[index] = row.Label
	}

	return labels
}
