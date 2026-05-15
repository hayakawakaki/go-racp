package transport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	accountdomain "github.com/hayakawakaki/go-racp/internal/account/domain"
	"github.com/hayakawakaki/go-racp/internal/account/transport/middleware"
	newsapp "github.com/hayakawakaki/go-racp/internal/news/app"
	"github.com/hayakawakaki/go-racp/internal/news/domain"
	newsinfra "github.com/hayakawakaki/go-racp/internal/news/infra"
	"github.com/hayakawakaki/go-racp/server/config"
)

type fakeService struct {
	items      map[int64]newsapp.NewsItem
	categories domain.CategoryResolver
}

func newFakeService() *fakeService {
	return &fakeService{
		items: map[int64]newsapp.NewsItem{},
		categories: domain.NewCategoryResolver([]domain.Category{
			{Key: "Announcement", Display: "Announcement"},
			{Key: "Patch", Display: "Patch Notes"},
		}),
	}
}

func (s *fakeService) Categories() domain.CategoryResolver { return s.categories }

func (s *fakeService) Create(context.Context, string, string, string) (int64, error) {
	return 0, nil
}

func (s *fakeService) Update(context.Context, int64, string, string, string) error {
	return nil
}

func (s *fakeService) Delete(context.Context, int64) error { return nil }

func (s *fakeService) GetByID(_ context.Context, id int64) (newsapp.NewsItem, error) {
	item, ok := s.items[id]
	if !ok {
		return newsapp.NewsItem{}, domain.ErrNotFound
	}

	return item, nil
}

func (s *fakeService) List(context.Context) ([]newsapp.NewsItem, error) {
	out := make([]newsapp.NewsItem, 0, len(s.items))
	for _, item := range s.items {
		out = append(out, item)
	}

	return out, nil
}

func (s *fakeService) ListByCategory(_ context.Context, category string) ([]newsapp.NewsItem, error) {
	out := make([]newsapp.NewsItem, 0)
	for _, item := range s.items {
		if item.Category == category {
			out = append(out, item)
		}
	}

	return out, nil
}

type fakeUsers struct {
	user *accountdomain.User
	err  error
}

func (f *fakeUsers) GetByID(context.Context, int) (*accountdomain.User, error) {
	return f.user, f.err
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newCanManageHandler(users userLookup, manageRoles []string) *Handler {
	resolver := accountdomain.NewRoleResolver(config.RolesConfig{
		"Moderator": 20,
		"Enforcer":  10,
	})

	return NewHandler(newFakeService(), newsinfra.NewRenderer(discardLogger()), HandlerConfig{
		Logger:      discardLogger(),
		Users:       users,
		Roles:       resolver,
		ManageRoles: manageRoles,
		General:     config.GeneralConfig{ServerName: "Test", Timezone: "UTC"},
	})
}

func TestHandler_CanManage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		session     *accountdomain.Session
		user        *accountdomain.User
		userErr     error
		name        string
		manageRoles []string
		want        bool
	}{
		{
			name: "no session",
			want: false,
		},
		{
			name:    "user lookup fails",
			session: &accountdomain.Session{UserID: 1},
			userErr: errors.New("db down"),
			want:    false,
		},
		{
			name:    "user nil without error",
			session: &accountdomain.Session{UserID: 1},
			user:    nil,
			want:    false,
		},
		{
			name:        "admin group always wins",
			session:     &accountdomain.Session{UserID: 1},
			user:        &accountdomain.User{ID: 1, GroupID: 99},
			manageRoles: nil,
			want:        true,
		},
		{
			name:        "role in manage list",
			session:     &accountdomain.Session{UserID: 1},
			user:        &accountdomain.User{ID: 1, GroupID: 20},
			manageRoles: []string{"Moderator"},
			want:        true,
		},
		{
			name:        "role not in manage list",
			session:     &accountdomain.Session{UserID: 1},
			user:        &accountdomain.User{ID: 1, GroupID: 10},
			manageRoles: []string{"Moderator"},
			want:        false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			users := &fakeUsers{user: tt.user, err: tt.userErr}
			h := newCanManageHandler(users, tt.manageRoles)

			req := httptest.NewRequest(http.MethodGet, "/news", http.NoBody)
			if tt.session != nil {
				req = req.WithContext(middleware.ContextWithSession(req.Context(), tt.session))
			}

			if got := h.canManage(req); got != tt.want {
				t.Errorf("canManage = %v, want %v", got, tt.want)
			}
		})
	}
}

func newJSONHandler(svc *fakeService) *Handler {
	return NewHandler(svc, newsinfra.NewRenderer(discardLogger()), HandlerConfig{
		Logger:  discardLogger(),
		General: config.GeneralConfig{ServerName: "Test", Timezone: "UTC"},
	})
}

func TestHandler_JSONList(t *testing.T) {
	t.Parallel()
	svc := newFakeService()
	svc.items[1] = newsapp.NewsItem{
		ID:        1,
		Title:     "Hello",
		Body:      "## Body",
		Category:  "Announcement",
		CreatedAt: time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC),
	}
	h := newJSONHandler(svc)

	rr := httptest.NewRecorder()
	h.jsonList(rr, httptest.NewRequest(http.MethodGet, "/api/v1/news", http.NoBody))

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}

	var got []newsJSON
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0].Title != "Hello" || got[0].Body != "## Body" {
		t.Errorf("body = %+v", got)
	}
}

func TestHandler_JSONList_EmptyArrayNotNull(t *testing.T) {
	t.Parallel()
	h := newJSONHandler(newFakeService())

	rr := httptest.NewRecorder()
	h.jsonList(rr, httptest.NewRequest(http.MethodGet, "/api/v1/news", http.NoBody))

	body := strings.TrimSpace(rr.Body.String())
	if body != "[]" {
		t.Errorf("empty response = %q, want []", body)
	}
}

func TestHandler_JSONList_FiltersByCategory(t *testing.T) {
	t.Parallel()
	svc := newFakeService()
	svc.items[1] = newsapp.NewsItem{ID: 1, Title: "A", Category: "Announcement"}
	svc.items[2] = newsapp.NewsItem{ID: 2, Title: "B", Category: "Patch"}
	h := newJSONHandler(svc)

	rr := httptest.NewRecorder()
	h.jsonList(rr, httptest.NewRequest(http.MethodGet, "/api/v1/news?category=Patch", http.NoBody))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	var got []newsJSON
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0].Title != "B" {
		t.Errorf("got = %+v, want one item with Title B", got)
	}
}

func TestHandler_JSONList_UnknownCategoryReturnsAll(t *testing.T) {
	t.Parallel()
	svc := newFakeService()
	svc.items[1] = newsapp.NewsItem{ID: 1, Title: "A", Category: "Announcement"}
	svc.items[2] = newsapp.NewsItem{ID: 2, Title: "B", Category: "Patch"}
	h := newJSONHandler(svc)

	rr := httptest.NewRecorder()
	h.jsonList(rr, httptest.NewRequest(http.MethodGet, "/api/v1/news?category=Bogus", http.NoBody))

	var got []newsJSON
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len(got) = %d, want 2 (unknown category should silently return all)", len(got))
	}
}

func TestHandler_JSONGet_NotFound(t *testing.T) {
	t.Parallel()
	h := newJSONHandler(newFakeService())

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/news/9999", http.NoBody)
	req.SetPathValue("id", "9999")
	h.jsonGet(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["error"] != "news not found" {
		t.Errorf("error = %q, want \"news not found\"", body["error"])
	}
}

func TestHandler_JSONGet_InvalidIDReturns404(t *testing.T) {
	t.Parallel()
	h := newJSONHandler(newFakeService())

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/news/abc", http.NoBody)
	req.SetPathValue("id", "abc")
	h.jsonGet(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

func TestHandler_JSONGet_Success(t *testing.T) {
	t.Parallel()
	svc := newFakeService()
	svc.items[42] = newsapp.NewsItem{
		ID: 42, Title: "Hello", Body: "## Body", Category: "Announcement",
		CreatedAt: time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC),
	}
	h := newJSONHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/news/42", http.NoBody)
	req.SetPathValue("id", "42")
	h.jsonGet(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	var got newsJSON
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID != 42 || got.Title != "Hello" || got.Category != "Announcement" {
		t.Errorf("body = %+v", got)
	}
}
