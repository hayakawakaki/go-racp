package transport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/a-h/templ"
	itemdomain "github.com/hayakawakaki/go-racp/internal/features/item/domain"
	"github.com/hayakawakaki/go-racp/internal/features/mob/app"
	"github.com/hayakawakaki/go-racp/internal/features/mob/domain"
	mobstate "github.com/hayakawakaki/go-racp/internal/features/mob/transport/state"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
	"github.com/hayakawakaki/go-racp/internal/platform/refdata"
	"github.com/hayakawakaki/go-racp/server/config"
	mobtpl "github.com/hayakawakaki/go-racp/themes/default/features/mob/transport"
	_ "github.com/hayakawakaki/go-racp/themes/default/platform/httpx"
)

type stubTheme struct{}

func (stubTheme) MobDetailPage(layout httpx.Layout, state mobstate.DetailState) templ.Component {
	return mobtpl.MobDetailPage(layout, state)
}

func (stubTheme) MobListPage(layout httpx.Layout, state mobstate.ListState) templ.Component {
	return mobtpl.MobListPage(layout, state)
}

func (stubTheme) MobNotFoundPage(layout httpx.Layout, id string) templ.Component {
	return mobtpl.MobNotFoundPage(layout, id)
}

func (stubTheme) MobEmptyDatabasePage(layout httpx.Layout) templ.Component {
	return mobtpl.MobEmptyDatabasePage(layout)
}

func (stubTheme) MobReloadSuccess(status app.ServiceStatus) templ.Component {
	return mobtpl.MobReloadSuccess(status)
}

func (stubTheme) MobReloadFailure(message string) templ.Component {
	return mobtpl.MobReloadFailure(message)
}

func (stubTheme) MobReloadConflict() templ.Component {
	return mobtpl.MobReloadConflict()
}

//nolint:govet // fine for test
type fakeMobService struct {
	mobsByID    map[int]*domain.Mob
	getErr      error
	listErr     error
	reloadErr   error
	listResp    app.MobPage
	gotListArg  app.ListQuery
	statusResp  app.ServiceStatus
	reloadCalls int
}

func newFakeMobService() *fakeMobService {
	return &fakeMobService{mobsByID: map[int]*domain.Mob{}}
}

func (s *fakeMobService) Get(_ context.Context, id int) (*domain.Mob, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	mob, ok := s.mobsByID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}

	return mob, nil
}

func (s *fakeMobService) List(_ context.Context, query app.ListQuery) (app.MobPage, error) {
	s.gotListArg = query
	if s.listErr != nil {
		return app.MobPage{}, s.listErr
	}

	return s.listResp, nil
}

func (s *fakeMobService) Reload(_ context.Context) error {
	s.reloadCalls++

	return s.reloadErr
}

func (s *fakeMobService) Status() app.ServiceStatus { return s.statusResp }

type fakeItemLookup struct {
	byAegis   map[string]*itemdomain.Item
	gotArgs   []string
	notLoaded bool
}

func (l *fakeItemLookup) LookupByAegis(aegis string) *itemdomain.Item {
	l.gotArgs = append(l.gotArgs, aegis)

	return l.byAegis[aegis]
}

func (l *fakeItemLookup) Loaded() bool { return !l.notLoaded }

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestHandler(svc mobService) *Handler {
	return NewHandler(svc, HandlerConfig{
		Logger:  discardLogger(),
		General: config.GeneralConfig{ServerName: "Test", Timezone: "UTC"},
		Theme:   stubTheme{},
	})
}

func newTestHandlerWithLookup(svc mobService, lookup ItemLookup) *Handler {
	return NewHandler(svc, HandlerConfig{
		Logger:       discardLogger(),
		ItemLookupFn: func() ItemLookup { return lookup },
		General:      config.GeneralConfig{ServerName: "Test", Timezone: "UTC"},
		Theme:        stubTheme{},
	})
}

func TestHandler_ShowList_EmptySnapshotReturnsEmptyDatabasePage(t *testing.T) {
	t.Parallel()

	svc := newFakeMobService()
	svc.listErr = domain.ErrEmptySnapshot
	h := newTestHandler(svc)

	rr := httptest.NewRecorder()
	h.showList(rr, httptest.NewRequest(http.MethodGet, "/mobs", http.NoBody))

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
	if !strings.Contains(rr.Body.String(), "No monsters loaded") {
		t.Errorf("body missing empty-database marker:\n%s", rr.Body.String())
	}
}

func TestHandler_ShowList_RendersResults(t *testing.T) {
	t.Parallel()

	mob := &domain.Mob{ID: 1002, AegisName: "PORING", Name: "Poring"}
	svc := newFakeMobService()
	svc.listResp = app.MobPage{
		Mobs: []*domain.Mob{mob}, Total: 1, Page: 1, PerPage: app.DefaultPerPage, TotalPages: 1,
	}
	h := newTestHandler(svc)

	rr := httptest.NewRecorder()
	h.showList(rr, httptest.NewRequest(http.MethodGet, "/mobs", http.NoBody))

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Poring") {
		t.Errorf("body missing mob name:\n%s", rr.Body.String())
	}
}

func TestHandler_ShowList_ServiceErrorReturns500(t *testing.T) {
	t.Parallel()

	svc := newFakeMobService()
	svc.listErr = errors.New("boom")
	h := newTestHandler(svc)

	rr := httptest.NewRecorder()
	h.showList(rr, httptest.NewRequest(http.MethodGet, "/mobs", http.NoBody))

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
	}{
		{
			name:        "defaults when no params",
			url:         "/mobs",
			wantPage:    1,
			wantPerPage: app.DefaultPerPage,
		},
		{
			name:        "page param",
			url:         "/mobs?page=3",
			wantPage:    3,
			wantPerPage: app.DefaultPerPage,
		},
		{
			name:        "negative page falls back",
			url:         "/mobs?page=-5",
			wantPage:    1,
			wantPerPage: app.DefaultPerPage,
		},
		{
			name:        "non-numeric page falls back",
			url:         "/mobs?page=abc",
			wantPage:    1,
			wantPerPage: app.DefaultPerPage,
		},
		{
			name:        "query is forwarded",
			url:         "/mobs?q=poring",
			wantPage:    1,
			wantQuery:   "poring",
			wantPerPage: app.DefaultPerPage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := newFakeMobService()
			h := newTestHandler(svc)

			rr := httptest.NewRecorder()
			h.showList(rr, httptest.NewRequest(http.MethodGet, tt.url, http.NoBody))

			if svc.gotListArg.Page != tt.wantPage {
				t.Errorf("Page = %d, want %d", svc.gotListArg.Page, tt.wantPage)
			}
			if svc.gotListArg.Query != tt.wantQuery {
				t.Errorf("Query = %q, want %q", svc.gotListArg.Query, tt.wantQuery)
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
		{name: "missing mob", id: "9999"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := newFakeMobService()
			h := newTestHandler(svc)

			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/mobs/"+tt.id, http.NoBody)
			req.SetPathValue("id", tt.id)
			h.showDetail(rr, req)

			if rr.Code != http.StatusNotFound {
				t.Errorf("status = %d, want 404", rr.Code)
			}
			if !strings.Contains(rr.Body.String(), "Monster not found") {
				t.Errorf("body missing not-found marker:\n%s", rr.Body.String())
			}
		})
	}
}

func TestHandler_ShowDetail_EmptySnapshotRendersNotFound(t *testing.T) {
	t.Parallel()

	svc := newFakeMobService()
	svc.getErr = domain.ErrEmptySnapshot
	h := newTestHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/mobs/1002", http.NoBody)
	req.SetPathValue("id", "1002")
	h.showDetail(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

func TestHandler_ShowDetail_ServiceErrorReturns500(t *testing.T) {
	t.Parallel()

	svc := newFakeMobService()
	svc.getErr = errors.New("db down")
	h := newTestHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/mobs/1002", http.NoBody)
	req.SetPathValue("id", "1002")
	h.showDetail(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
}

func TestHandler_ShowDetail_RendersMob(t *testing.T) {
	t.Parallel()

	svc := newFakeMobService()
	svc.mobsByID[1002] = &domain.Mob{
		ID:        1002,
		AegisName: "PORING",
		Name:      "Poring",
		Level:     1,
		HP:        50,
		Race:      domain.RacePlant,
		Element:   domain.ElementWater,
		Size:      domain.SizeSmall,
	}
	h := newTestHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/mobs/1002", http.NoBody)
	req.SetPathValue("id", "1002")
	h.showDetail(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Poring") {
		t.Errorf("body missing mob name:\n%s", body)
	}
}

func TestHandler_ShowDetail_DropsUseItemLookupWhenAvailable(t *testing.T) {
	t.Parallel()

	svc := newFakeMobService()
	svc.mobsByID[1002] = &domain.Mob{
		ID:        1002,
		AegisName: "PORING",
		Name:      "Poring",
		Race:      domain.RacePlant,
		Element:   domain.ElementWater,
		Size:      domain.SizeSmall,
		Drops:     []domain.MobDrop{{ItemAegis: "Red_Potion", Rate: 1000}},
	}
	lookup := &fakeItemLookup{byAegis: map[string]*itemdomain.Item{
		"Red_Potion": {ID: 501, AegisName: "Red_Potion", ClientName: "Red Potion", Image: "red_potion"},
	}}
	h := newTestHandlerWithLookup(svc, lookup)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/mobs/1002", http.NoBody)
	req.SetPathValue("id", "1002")
	h.showDetail(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if len(lookup.gotArgs) != 1 || lookup.gotArgs[0] != "Red_Potion" {
		t.Errorf("lookup args = %v, want [Red_Potion]", lookup.gotArgs)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Red Potion") {
		t.Errorf("body missing resolved client name:\n%s", body)
	}
	if !strings.Contains(body, "/items/501") {
		t.Errorf("body missing item link to /items/501:\n%s", body)
	}
}

func TestHandler_ShowDetail_ItemDBNotLoadedOmitsDrops(t *testing.T) {
	t.Parallel()

	svc := newFakeMobService()
	svc.mobsByID[1002] = &domain.Mob{
		ID:        1002,
		AegisName: "PORING",
		Name:      "Poring",
		Race:      domain.RacePlant,
		Element:   domain.ElementWater,
		Size:      domain.SizeSmall,
		Drops:     []domain.MobDrop{{ItemAegis: "Red_Potion", Rate: 1000}},
	}
	lookup := &fakeItemLookup{notLoaded: true}
	h := newTestHandlerWithLookup(svc, lookup)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/mobs/1002", http.NoBody)
	req.SetPathValue("id", "1002")
	h.showDetail(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if body := rr.Body.String(); strings.Contains(body, "Red_Potion") {
		t.Errorf("drops rendered when item DB not loaded:\n%s", body)
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
			svc := newFakeMobService()
			h := newTestHandler(svc)

			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/mobs/"+tt.id, http.NoBody)
			req.SetPathValue("id", tt.id)
			h.apiDetail(rr, req)

			if rr.Code != http.StatusNotFound {
				t.Errorf("status = %d, want 404", rr.Code)
			}
			var body apiError
			if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if body.Error != "mob not found" {
				t.Errorf("error = %q", body.Error)
			}
			if body.ID != 0 {
				t.Errorf("ID = %d, want 0 for invalid input", body.ID)
			}
		})
	}
}

func TestHandler_APIDetail_NotFoundPreservesID(t *testing.T) {
	t.Parallel()

	svc := newFakeMobService()
	h := newTestHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/mobs/9999", http.NoBody)
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
		t.Errorf("ID = %d, want 9999", body.ID)
	}
}

func TestHandler_APIDetail_EmptySnapshotReturns404(t *testing.T) {
	t.Parallel()

	svc := newFakeMobService()
	svc.getErr = domain.ErrEmptySnapshot
	h := newTestHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/mobs/1002", http.NoBody)
	req.SetPathValue("id", "1002")
	h.apiDetail(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

func TestHandler_APIDetail_ServiceErrorReturns500(t *testing.T) {
	t.Parallel()

	svc := newFakeMobService()
	svc.getErr = errors.New("db down")
	h := newTestHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/mobs/1002", http.NoBody)
	req.SetPathValue("id", "1002")
	h.apiDetail(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
}

func TestHandler_APIDetail_Success(t *testing.T) {
	t.Parallel()

	svc := newFakeMobService()
	svc.mobsByID[1002] = &domain.Mob{
		ID:        1002,
		AegisName: "PORING",
		Name:      "Poring",
		Race:      domain.RacePlant,
		Element:   domain.ElementWater,
		Size:      domain.SizeSmall,
	}
	h := newTestHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/mobs/1002", http.NoBody)
	req.SetPathValue("id", "1002")
	h.apiDetail(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}

	var got app.MobDTO
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID != 1002 || got.AegisName != "PORING" || got.Race != "Plant" {
		t.Errorf("body = %+v", got)
	}
}

func TestHandler_DoReload_Success(t *testing.T) {
	t.Parallel()

	svc := newFakeMobService()
	svc.statusResp = app.ServiceStatus{MobsLoaded: 7, LastReload: time.Now().Format(time.RFC3339)}
	h := newTestHandler(svc)

	rr := httptest.NewRecorder()
	h.doReload(rr, httptest.NewRequest(http.MethodPost, "/admin/mobs/reload", http.NoBody))

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if svc.reloadCalls != 1 {
		t.Errorf("reload calls = %d, want 1", svc.reloadCalls)
	}
}

func TestHandler_DoReload_ConflictReturns409(t *testing.T) {
	t.Parallel()

	svc := newFakeMobService()
	svc.reloadErr = refdata.ErrReloadConflict
	h := newTestHandler(svc)

	rr := httptest.NewRecorder()
	h.doReload(rr, httptest.NewRequest(http.MethodPost, "/admin/mobs/reload", http.NoBody))

	if rr.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rr.Code)
	}
}

func TestHandler_DoReload_FailureReturns500(t *testing.T) {
	t.Parallel()

	svc := newFakeMobService()
	svc.reloadErr = errors.New("disk full")
	h := newTestHandler(svc)

	rr := httptest.NewRecorder()
	h.doReload(rr, httptest.NewRequest(http.MethodPost, "/admin/mobs/reload", http.NoBody))

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
}

func TestNewHandler_NilLoggerFallsBackToDefault(t *testing.T) {
	t.Parallel()

	h := NewHandler(newFakeMobService(), HandlerConfig{})
	if h.logger == nil {
		t.Errorf("logger is nil, want fallback slog.Default")
	}
}

func TestHandler_CurrentItemLookup_NilWhenFnUnset(t *testing.T) {
	t.Parallel()

	h := NewHandler(newFakeMobService(), HandlerConfig{})
	if got := h.currentItemLookup(); got != nil {
		t.Errorf("currentItemLookup() = %v, want nil", got)
	}
}

func TestResolveDrops_NilLookupKeepsAegis(t *testing.T) {
	t.Parallel()

	rows := resolveDrops([]domain.MobDrop{{ItemAegis: "Red_Potion", Rate: 1000}}, nil, false, config.RatesConfig{})
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}
	if rows[0].Aegis != "Red_Potion" || rows[0].ItemID != 0 {
		t.Errorf("rows[0] = %+v, want aegis with zero ItemID", rows[0])
	}
}

func TestResolveDrops_LookupHitFillsItemFields(t *testing.T) {
	t.Parallel()

	lookup := &fakeItemLookup{byAegis: map[string]*itemdomain.Item{
		"Red_Potion": {ID: 501, AegisName: "Red_Potion", ClientName: "Red Potion", Image: "red_potion", Slots: 0},
	}}
	rows := resolveDrops([]domain.MobDrop{{ItemAegis: "Red_Potion", Rate: 1000}}, lookup, false, config.RatesConfig{})
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}
	if rows[0].ItemID != 501 || rows[0].ClientName != "Red Potion" || rows[0].Image != "red_potion" {
		t.Errorf("rows[0] = %+v", rows[0])
	}
}

func TestResolveDrops_LookupMissPreservesAegis(t *testing.T) {
	t.Parallel()

	lookup := &fakeItemLookup{byAegis: map[string]*itemdomain.Item{}}
	rows := resolveDrops([]domain.MobDrop{{ItemAegis: "Unknown_Item", Rate: 5}}, lookup, false, config.RatesConfig{})
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}
	if rows[0].Aegis != "Unknown_Item" || rows[0].ItemID != 0 {
		t.Errorf("rows[0] = %+v, want aegis-only row", rows[0])
	}
}

func TestResolveDrops_EmptyInputReturnsNil(t *testing.T) {
	t.Parallel()

	if got := resolveDrops(nil, nil, false, config.RatesConfig{}); got != nil {
		t.Errorf("resolveDrops(nil, nil) = %v, want nil", got)
	}
}

func TestBuildExp_ZeroSourcesReturnsNil(t *testing.T) {
	t.Parallel()

	if got := buildExp(&domain.Mob{}, config.RatesConfig{}); got != nil {
		t.Errorf("buildExp = %v, want nil for zero exp", got)
	}
}

func TestBuildExp_IncludesMvpExpWhenSet(t *testing.T) {
	t.Parallel()

	rows := buildExp(&domain.Mob{BaseExp: 100, JobExp: 50, MvpExp: 25}, config.RatesConfig{})
	if len(rows) != 3 {
		t.Fatalf("rows len = %d, want 3 (base + job + mvp)", len(rows))
	}
	if rows[2].Label != "MVP Exp" {
		t.Errorf("rows[2].Label = %q, want MVP Exp", rows[2].Label)
	}
}

func TestBuildExp_OmitsMvpRowWhenAbsent(t *testing.T) {
	t.Parallel()

	rows := buildExp(&domain.Mob{BaseExp: 100, JobExp: 50}, config.RatesConfig{})
	if len(rows) != 2 {
		t.Fatalf("rows len = %d, want 2", len(rows))
	}
	for _, row := range rows {
		if row.Label == "MVP Exp" {
			t.Errorf("MVP Exp row should be omitted when MvpExp=0")
		}
	}
}

func TestDropRow_DisplayName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
		row  mobstate.DropRow
	}{
		{name: "client name preferred", row: mobstate.DropRow{Aegis: "A", ClientName: "Pretty"}, want: "Pretty"},
		{name: "aegis fallback when client name empty", row: mobstate.DropRow{Aegis: "Fallback"}, want: "Fallback"},
		{name: "slots append bracket", row: mobstate.DropRow{ClientName: "Sword", Slots: 3}, want: "Sword [3]"},
		{name: "no bracket when zero slots", row: mobstate.DropRow{ClientName: "Card"}, want: "Card"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.row.DisplayName(); got != tt.want {
				t.Errorf("DisplayName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestScaleRate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		value      int
		multiplier int
		want       int
	}{
		{name: "identity x1", value: 1000, multiplier: 100, want: 1000},
		{name: "double", value: 1000, multiplier: 200, want: 2000},
		{name: "half", value: 1000, multiplier: 50, want: 500},
		{name: "truncates toward zero", value: 15, multiplier: 110, want: 16},
		{name: "zero value", value: 0, multiplier: 200, want: 0},
		{name: "zero multiplier", value: 1000, multiplier: 0, want: 0},
		{name: "negative value", value: -5, multiplier: 200, want: 0},
		{name: "overflow saturates", value: math.MaxInt, multiplier: 200, want: math.MaxInt / 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := scaleRate(tt.value, tt.multiplier); got != tt.want {
				t.Errorf("scaleRate(%d, %d) = %d, want %d", tt.value, tt.multiplier, got, tt.want)
			}
		})
	}
}

func TestCategoryRate(t *testing.T) {
	t.Parallel()

	rates := config.RatesConfig{
		DropRateCommon:  1,
		DropRateHeal:    2,
		DropRateUsable:  3,
		DropRateEquip:   4,
		DropRateCard:    5,
		DropRateCardMVP: 6,
	}
	tests := []struct {
		name     string
		itemType itemdomain.ItemType
		isMVP    bool
		want     int
	}{
		{name: "healing", itemType: itemdomain.ItemTypeHealing, want: 2},
		{name: "usable", itemType: itemdomain.ItemTypeUsable, want: 3},
		{name: "delayconsume maps to usable", itemType: itemdomain.ItemTypeDelayConsume, want: 3},
		{name: "cash maps to usable", itemType: itemdomain.ItemTypeCash, want: 3},
		{name: "weapon maps to equip", itemType: itemdomain.ItemTypeWeapon, want: 4},
		{name: "armor maps to equip", itemType: itemdomain.ItemTypeArmor, want: 4},
		{name: "shadowgear maps to equip", itemType: itemdomain.ItemTypeShadowGear, want: 4},
		{name: "card normal", itemType: itemdomain.ItemTypeCard, isMVP: false, want: 5},
		{name: "card mvp", itemType: itemdomain.ItemTypeCard, isMVP: true, want: 6},
		{name: "etc maps to common", itemType: itemdomain.ItemTypeEtc, want: 1},
		{name: "ammo maps to common", itemType: itemdomain.ItemTypeAmmo, want: 1},
		{name: "unknown maps to common", itemType: itemdomain.ItemTypeUnknown, want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := categoryRate(tt.itemType, tt.isMVP, rates); got != tt.want {
				t.Errorf("categoryRate(%v, mvp=%v) = %d, want %d", tt.itemType, tt.isMVP, got, tt.want)
			}
		})
	}
}

func TestResolveDrops_AppliesCategoryMultiplier(t *testing.T) {
	t.Parallel()

	lookup := &fakeItemLookup{byAegis: map[string]*itemdomain.Item{
		"Red_Potion":  {ID: 501, ClientName: "Red Potion", Type: itemdomain.ItemTypeHealing},
		"Poring_Card": {ID: 4001, ClientName: "Poring Card", Type: itemdomain.ItemTypeCard},
	}}
	rates := config.RatesConfig{DropRateHeal: 200, DropRateCard: 500, DropRateCommon: 100}
	drops := []domain.MobDrop{
		{ItemAegis: "Red_Potion", Rate: 100},
		{ItemAegis: "Poring_Card", Rate: 50},
		{ItemAegis: "Unknown_Item", Rate: 30},
	}

	rows := resolveDrops(drops, lookup, false, rates)

	if rows[0].Rate != 200 {
		t.Errorf("heal drop rate = %d, want 200 (100 * x2)", rows[0].Rate)
	}
	if rows[1].Rate != 250 {
		t.Errorf("card drop rate = %d, want 250 (50 * x5)", rows[1].Rate)
	}
	if rows[2].Rate != 30 {
		t.Errorf("unknown drop rate = %d, want 30 (common x1 fallback)", rows[2].Rate)
	}
}

func TestResolveDrops_CapsAt100Percent(t *testing.T) {
	t.Parallel()

	lookup := &fakeItemLookup{byAegis: map[string]*itemdomain.Item{
		"Jellopy": {ID: 909, ClientName: "Jellopy", Type: itemdomain.ItemTypeEtc},
	}}
	rates := config.RatesConfig{DropRateCommon: 1000}

	rows := resolveDrops([]domain.MobDrop{{ItemAegis: "Jellopy", Rate: 5000}}, lookup, false, rates)

	if rows[0].Rate != 10000 {
		t.Errorf("rate = %d, want 10000 (50%% * x10 capped at 100%%)", rows[0].Rate)
	}
}

func TestResolveDrops_MvpCardUsesMvpRate(t *testing.T) {
	t.Parallel()

	lookup := &fakeItemLookup{byAegis: map[string]*itemdomain.Item{
		"Baphomet_Card": {ID: 4147, ClientName: "Baphomet Card", Type: itemdomain.ItemTypeCard},
	}}
	rates := config.RatesConfig{DropRateCard: 100, DropRateCardMVP: 300}
	drop := []domain.MobDrop{{ItemAegis: "Baphomet_Card", Rate: 10}}

	normal := resolveDrops(drop, lookup, false, rates)
	mvp := resolveDrops(drop, lookup, true, rates)

	if normal[0].Rate != 10 {
		t.Errorf("normal card rate = %d, want 10 (x1)", normal[0].Rate)
	}
	if mvp[0].Rate != 30 {
		t.Errorf("mvp card rate = %d, want 30 (x3)", mvp[0].Rate)
	}
}

func TestBuildExp_AppliesMultipliers(t *testing.T) {
	t.Parallel()

	rates := config.RatesConfig{ExpRate: 200, JobRate: 300}
	rows := buildExp(&domain.Mob{BaseExp: 100, JobExp: 50, MvpExp: 25}, rates)

	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rows))
	}
	if rows[0].Value != "200" {
		t.Errorf("Base Exp = %q, want 200 (100 * x2)", rows[0].Value)
	}
	if rows[1].Value != "150" {
		t.Errorf("Job Exp = %q, want 150 (50 * x3)", rows[1].Value)
	}
	if rows[2].Value != "50" {
		t.Errorf("MVP Exp = %q, want 50 (25 * x2 via ExpRate)", rows[2].Value)
	}
}

func TestHandler_APIDetail_AppliesRates(t *testing.T) {
	t.Parallel()

	svc := newFakeMobService()
	svc.mobsByID[1002] = &domain.Mob{
		ID:        1002,
		AegisName: "PORING",
		Name:      "Poring",
		Race:      domain.RacePlant,
		Element:   domain.ElementWater,
		Size:      domain.SizeSmall,
		BaseExp:   100,
		JobExp:    50,
		Drops:     []domain.MobDrop{{ItemAegis: "Poring_Card", Rate: 100}},
	}
	lookup := &fakeItemLookup{byAegis: map[string]*itemdomain.Item{
		"Poring_Card": {ID: 4001, AegisName: "Poring_Card", ClientName: "Poring Card", Type: itemdomain.ItemTypeCard},
	}}
	h := NewHandler(svc, HandlerConfig{
		Logger:       discardLogger(),
		ItemLookupFn: func() ItemLookup { return lookup },
		General: config.GeneralConfig{
			ServerName: "Test",
			Timezone:   "UTC",
			Rates:      config.RatesConfig{ExpRate: 200, JobRate: 300, DropRateCard: 500},
		},
		Theme: stubTheme{},
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/mobs/1002", http.NoBody)
	req.SetPathValue("id", "1002")
	h.apiDetail(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var got app.MobDTO
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.BaseExp != 200 {
		t.Errorf("base_exp = %d, want 200 (100 * x2)", got.BaseExp)
	}
	if got.JobExp != 150 {
		t.Errorf("job_exp = %d, want 150 (50 * x3)", got.JobExp)
	}
	if len(got.Drops) != 1 || got.Drops[0].Rate != 500 {
		t.Errorf("drops = %+v, want one drop at rate 500 (100 * x5)", got.Drops)
	}
}
