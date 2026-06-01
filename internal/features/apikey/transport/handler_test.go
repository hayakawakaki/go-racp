package transport

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/hayakawakaki/go-racp/internal/features/apikey/domain"
	"github.com/hayakawakaki/go-racp/internal/features/apikey/transport/state"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
	"github.com/hayakawakaki/go-racp/server/config"
)

type stubRenderer struct {
	last    state.ListState
	pages   int
	content int
}

func emptyComponent() templ.Component {
	return templ.ComponentFunc(func(context.Context, io.Writer) error { return nil })
}

func (s *stubRenderer) APIKeysPage(_ httpx.Layout, st state.ListState) templ.Component {
	s.pages++
	s.last = st

	return emptyComponent()
}

func (s *stubRenderer) APIKeysContent(st state.ListState) templ.Component {
	s.content++
	s.last = st

	return emptyComponent()
}

type fakeService struct {
	generateKey *domain.APIKey
	generateErr error
	listErr     error
	revokeErr   error
	generateRaw string
	keys        []domain.APIKey
	tiers       []domain.Tier
	revoked     []int64
}

func (s *fakeService) Generate(context.Context, string, string) (string, *domain.APIKey, error) {
	return s.generateRaw, s.generateKey, s.generateErr
}

func (s *fakeService) List(context.Context) ([]domain.APIKey, error) {
	return s.keys, s.listErr
}

func (s *fakeService) Revoke(_ context.Context, id int64) error {
	s.revoked = append(s.revoked, id)

	return s.revokeErr
}

func (s *fakeService) Tiers() []domain.Tier {
	return s.tiers
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestHandler(svc *fakeService) (*Handler, *stubRenderer) {
	renderer := &stubRenderer{}
	handler := NewHandler(svc, HandlerConfig{
		General: config.GeneralConfig{ServerName: "Test", Timezone: "UTC"},
		Theme:   renderer,
		Logger:  discardLogger(),
	})

	return handler, renderer
}

func TestHandler_ShowList(t *testing.T) {
	t.Parallel()
	svc := &fakeService{
		keys:  []domain.APIKey{{ID: 1, Name: "deploy bot", RateTier: "Standard"}},
		tiers: []domain.Tier{{Name: "Standard"}},
	}
	handler, renderer := newTestHandler(svc)

	recorder := httptest.NewRecorder()
	handler.showList(recorder, httptest.NewRequest(http.MethodGet, "/admin/api-keys", http.NoBody))

	if recorder.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", recorder.Code)
	}
	if renderer.pages != 1 {
		t.Errorf("APIKeysPage calls = %d, want 1", renderer.pages)
	}
	if len(renderer.last.Keys) != 1 || len(renderer.last.Tiers) != 1 {
		t.Errorf("state keys=%d tiers=%d, want 1 and 1", len(renderer.last.Keys), len(renderer.last.Tiers))
	}
}

func TestHandler_ShowList_HTMXRendersContent(t *testing.T) {
	t.Parallel()
	handler, renderer := newTestHandler(&fakeService{})

	req := httptest.NewRequest(http.MethodGet, "/admin/api-keys", http.NoBody)
	req.Header.Set("HX-Request", "true")
	recorder := httptest.NewRecorder()
	handler.showList(recorder, req)

	if renderer.content != 1 {
		t.Errorf("APIKeysContent calls = %d, want 1", renderer.content)
	}
	if renderer.pages != 0 {
		t.Errorf("APIKeysPage calls = %d, want 0 for htmx request", renderer.pages)
	}
}

func TestHandler_ShowList_ListErrorReturns500(t *testing.T) {
	t.Parallel()
	handler, _ := newTestHandler(&fakeService{listErr: errors.New("db down")})

	recorder := httptest.NewRecorder()
	handler.showList(recorder, httptest.NewRequest(http.MethodGet, "/admin/api-keys", http.NoBody))

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", recorder.Code)
	}
}

func TestHandler_Create_Success(t *testing.T) {
	t.Parallel()
	svc := &fakeService{
		generateRaw: "rawsecret",
		generateKey: &domain.APIKey{ID: 1, Name: "deploy bot", RateTier: "Standard"},
	}
	handler, renderer := newTestHandler(svc)

	recorder := httptest.NewRecorder()
	handler.create(recorder, formRequest(url.Values{"name": {"deploy bot"}, "tier": {"Standard"}}))

	if recorder.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", recorder.Code)
	}
	if renderer.last.RevealedKey != "rawsecret" {
		t.Errorf("RevealedKey = %q, want rawsecret", renderer.last.RevealedKey)
	}
	if renderer.last.RevealedName != "deploy bot" {
		t.Errorf("RevealedName = %q, want deploy bot", renderer.last.RevealedName)
	}
}

func TestHandler_Create_ValidationErrorReRenders(t *testing.T) {
	t.Parallel()
	svc := &fakeService{
		generateErr: &domain.ValidationError{Fields: domain.FieldErrors{"name": "name is required"}},
	}
	handler, renderer := newTestHandler(svc)

	recorder := httptest.NewRecorder()
	handler.create(recorder, formRequest(url.Values{"name": {""}, "tier": {"Standard"}}))

	if recorder.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", recorder.Code)
	}
	if _, ok := renderer.last.Errors["name"]; !ok {
		t.Errorf("state Errors = %v, want a name entry", renderer.last.Errors)
	}
	if renderer.last.FormTier != "Standard" {
		t.Errorf("FormTier = %q, want Standard preserved", renderer.last.FormTier)
	}
}

func TestHandler_Create_InternalErrorReturns500(t *testing.T) {
	t.Parallel()
	handler, _ := newTestHandler(&fakeService{generateErr: errors.New("db down")})

	recorder := httptest.NewRecorder()
	handler.create(recorder, formRequest(url.Values{"name": {"deploy bot"}, "tier": {"Standard"}}))

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", recorder.Code)
	}
}

func TestHandler_Revoke(t *testing.T) {
	t.Parallel()

	tests := []struct {
		revokeErr  error
		name       string
		id         string
		wantStatus int
		wantCalled bool
	}{
		{name: "valid id revokes", id: "5", wantStatus: http.StatusOK, wantCalled: true},
		{name: "not found is treated as success", id: "5", revokeErr: domain.ErrKeyNotFound, wantStatus: http.StatusOK, wantCalled: true},
		{name: "non numeric id rejected", id: "abc", wantStatus: http.StatusBadRequest},
		{name: "zero id rejected", id: "0", wantStatus: http.StatusBadRequest},
		{name: "internal error surfaces", id: "5", revokeErr: errors.New("db down"), wantStatus: http.StatusInternalServerError, wantCalled: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := &fakeService{revokeErr: tt.revokeErr}
			handler, _ := newTestHandler(svc)

			req := httptest.NewRequest(http.MethodPost, "/admin/api-keys/"+tt.id+"/revoke", http.NoBody)
			req.SetPathValue("id", tt.id)
			recorder := httptest.NewRecorder()
			handler.revoke(recorder, req)

			if recorder.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}
			if got := len(svc.revoked) > 0; got != tt.wantCalled {
				t.Errorf("service Revoke called = %v, want %v", got, tt.wantCalled)
			}
		})
	}
}

func formRequest(values url.Values) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/admin/api-keys", strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return req
}
