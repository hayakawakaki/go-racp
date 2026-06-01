package transport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/hayakawakaki/go-racp/internal/features/guild/app"
	"github.com/hayakawakaki/go-racp/internal/features/guild/domain"
	guildstate "github.com/hayakawakaki/go-racp/internal/features/guild/transport/state"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
	guildtpl "github.com/hayakawakaki/go-racp/themes/default/features/guild/transport"
	_ "github.com/hayakawakaki/go-racp/themes/default/platform/httpx"
)

type stubTheme struct{}

func (stubTheme) GuildDetailPage(layout httpx.Layout, guildName string, state guildstate.DetailState) templ.Component {
	return guildtpl.GuildDetailPage(layout, guildName, state)
}

func (stubTheme) GuildDetailContent(state guildstate.DetailState) templ.Component {
	return guildtpl.GuildDetailContent(state)
}

type fakeService struct {
	getErr      error
	emblemErr   error
	emblemMime  string
	emblemData  []byte
	getResult   app.GuildDetail
	getID       int
	emblemID    int
	getCalls    int
	emblemCalls int
}

func (f *fakeService) Get(_ context.Context, id int) (app.GuildDetail, error) {
	f.getCalls++
	f.getID = id

	return f.getResult, f.getErr
}

func (f *fakeService) GetEmblem(_ context.Context, id int) (data []byte, mime string, err error) {
	f.emblemCalls++
	f.emblemID = id

	return f.emblemData, f.emblemMime, f.emblemErr
}

func newTestHandler(svc *fakeService) *Handler {
	return NewHandler(svc, HandlerConfig{Theme: stubTheme{}})
}

func newRequest(t *testing.T, method, target string, pathValues, headers map[string]string) *http.Request {
	t.Helper()
	r := httptest.NewRequest(method, target, http.NoBody)
	for k, v := range pathValues {
		r.SetPathValue(k, v)
	}
	for k, v := range headers {
		r.Header.Set(k, v)
	}

	return r
}

func TestHandler_ShowDetail_HappyPath(t *testing.T) {
	t.Parallel()
	guild := &domain.Guild{ID: 42, Name: "kaki", MasterName: "kaki", MasterCharID: 150000, GuildLevel: 5, MaxMember: 16}
	svc := &fakeService{getResult: app.GuildDetail{
		Guild:   guild,
		Members: []domain.Member{{Name: "kaki", PositionName: "Master", CharID: 150000, Position: 0}},
	}}
	h := newTestHandler(svc)

	r := newRequest(t, http.MethodGet, "/guilds/42", map[string]string{"id": "42"}, nil)
	w := httptest.NewRecorder()
	h.showDetail(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if svc.getID != 42 {
		t.Errorf("getID = %d, want 42", svc.getID)
	}
	body := w.Body.String()
	if !strings.Contains(body, "kaki") {
		t.Errorf("body missing guild name: %s", body)
	}
}

func TestHandler_ShowDetail_PaginatesMembers(t *testing.T) {
	t.Parallel()

	members := make([]domain.Member, 0, 25)
	for i := 1; i <= 25; i++ {
		members = append(members, domain.Member{
			Name:         fmt.Sprintf("Member%02d", i),
			PositionName: "Member",
			CharID:       1000 + i,
			Position:     1,
		})
	}
	svc := &fakeService{getResult: app.GuildDetail{
		Guild:   &domain.Guild{ID: 42, Name: "kaki", MaxMember: 56},
		Members: members,
	}}

	tests := []struct {
		name      string
		target    string
		wantBody  []string
		notInBody []string
	}{
		{
			name:      "first page shows members 1-10",
			target:    "/guilds/42?page=1",
			wantBody:  []string{"Member01", "Member10"},
			notInBody: []string{"Member11"},
		},
		{
			name:      "second page shows members 11-20",
			target:    "/guilds/42?page=2",
			wantBody:  []string{"Member11", "Member20"},
			notInBody: []string{"Member01"},
		},
		{
			name:     "out-of-range page clamps to the last page",
			target:   "/guilds/42?page=99",
			wantBody: []string{"Member21"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newRequest(t, http.MethodGet, tt.target, map[string]string{"id": "42"}, nil)
			w := httptest.NewRecorder()
			h := newTestHandler(svc)
			h.showDetail(w, r)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", w.Code)
			}
			body := w.Body.String()
			for _, want := range tt.wantBody {
				if !strings.Contains(body, want) {
					t.Errorf("body missing %q:\n%s", want, body)
				}
			}
			for _, absent := range tt.notInBody {
				if strings.Contains(body, absent) {
					t.Errorf("body should not contain %q:\n%s", absent, body)
				}
			}
		})
	}
}

func TestHandler_ShowDetail_HTMXReturnsFragment(t *testing.T) {
	t.Parallel()
	svc := &fakeService{getResult: app.GuildDetail{Guild: &domain.Guild{ID: 1, Name: "kaki"}}}
	h := newTestHandler(svc)

	r := newRequest(t, http.MethodGet, "/guilds/1", map[string]string{"id": "1"}, map[string]string{"HX-Request": "true"})
	w := httptest.NewRecorder()
	h.showDetail(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if strings.Contains(body, "<html") || strings.Contains(body, "<!DOCTYPE") {
		t.Errorf("HTMX response should be a fragment, got full page")
	}
}

func TestHandler_ShowDetail_InvalidIDReturns404(t *testing.T) {
	t.Parallel()
	svc := &fakeService{}
	h := newTestHandler(svc)

	r := newRequest(t, http.MethodGet, "/guilds/abc", map[string]string{"id": "abc"}, nil)
	w := httptest.NewRecorder()
	h.showDetail(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
	if svc.getCalls != 0 {
		t.Errorf("getCalls = %d, want 0 (service should not be invoked)", svc.getCalls)
	}
}

func TestHandler_ShowDetail_NotFoundReturns404(t *testing.T) {
	t.Parallel()
	svc := &fakeService{getErr: domain.ErrGuildNotFound}
	h := newTestHandler(svc)

	r := newRequest(t, http.MethodGet, "/guilds/9999", map[string]string{"id": "9999"}, nil)
	w := httptest.NewRecorder()
	h.showDetail(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestHandler_ShowDetail_ServiceErrorReturns500(t *testing.T) {
	t.Parallel()
	svc := &fakeService{getErr: errors.New("boom")}
	h := newTestHandler(svc)

	r := newRequest(t, http.MethodGet, "/guilds/1", map[string]string{"id": "1"}, nil)
	w := httptest.NewRecorder()
	h.showDetail(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestHandler_ShowEmblem_HappyPath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		mime string
		data []byte
	}{
		{name: "bmp", mime: "image/bmp", data: []byte{'B', 'M', 0x01, 0x02}},
		{name: "gif", mime: "image/gif", data: []byte("GIF89a\x00\x00")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := &fakeService{emblemData: tc.data, emblemMime: tc.mime}
			h := newTestHandler(svc)

			r := newRequest(t, http.MethodGet, "/guilds/1/emblem", map[string]string{"id": "1"}, nil)
			w := httptest.NewRecorder()
			h.showEmblem(w, r)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", w.Code)
			}
			if got := w.Header().Get("Content-Type"); got != tc.mime {
				t.Errorf("Content-Type = %q, want %q", got, tc.mime)
			}
			body, _ := io.ReadAll(w.Body)
			if !bytes.Equal(body, tc.data) {
				t.Errorf("body = %x, want %x", body, tc.data)
			}
		})
	}
}

func TestHandler_ShowEmblem_SentinelErrorsReturn404(t *testing.T) {
	t.Parallel()
	cases := []struct {
		sentinel error
		name     string
	}{
		{name: "guild not found", sentinel: domain.ErrGuildNotFound},
		{name: "emblem empty", sentinel: domain.ErrEmblemEmpty},
		{name: "unknown format", sentinel: domain.ErrEmblemUnknownFormat},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := &fakeService{emblemErr: tc.sentinel}
			h := newTestHandler(svc)

			r := newRequest(t, http.MethodGet, "/guilds/1/emblem", map[string]string{"id": "1"}, nil)
			w := httptest.NewRecorder()
			h.showEmblem(w, r)

			if w.Code != http.StatusNotFound {
				t.Errorf("status = %d, want 404", w.Code)
			}
		})
	}
}

func TestHandler_ShowEmblem_InvalidIDReturns404(t *testing.T) {
	t.Parallel()
	svc := &fakeService{}
	h := newTestHandler(svc)

	r := newRequest(t, http.MethodGet, "/guilds/abc/emblem", map[string]string{"id": "abc"}, nil)
	w := httptest.NewRecorder()
	h.showEmblem(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
	if svc.emblemCalls != 0 {
		t.Errorf("emblemCalls = %d, want 0", svc.emblemCalls)
	}
}

func TestHandler_ShowEmblem_UnknownErrorReturns500(t *testing.T) {
	t.Parallel()
	svc := &fakeService{emblemErr: errors.New("boom")}
	h := newTestHandler(svc)

	r := newRequest(t, http.MethodGet, "/guilds/1/emblem", map[string]string{"id": "1"}, nil)
	w := httptest.NewRecorder()
	h.showEmblem(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}
