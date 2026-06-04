package transport

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/a-h/templ"
	accdomain "github.com/hayakawakaki/go-racp/internal/features/account/domain"
	"github.com/hayakawakaki/go-racp/internal/features/account/transport/middleware"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
	notificationapp "github.com/hayakawakaki/go-racp/internal/platform/notification/app"
	"github.com/hayakawakaki/go-racp/internal/platform/notification/domain"
)

func init() {
	httpx.ActiveBase = func(_ httpx.Layout, _ string) templ.Component {
		return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
			return templ.GetChildren(ctx).Render(ctx, w)
		})
	}
}

var _ domain.Repository = (*fakeRepo)(nil)

type fakeRepo struct {
	link      string
	listItems []domain.Notification
	listTotal int
}

func (r *fakeRepo) Create(context.Context, domain.Notification) (domain.Notification, error) {
	return domain.Notification{}, nil
}

func (r *fakeRepo) RecentByAccount(context.Context, int, int) ([]domain.Notification, error) {
	return nil, nil
}

func (r *fakeRepo) ListPage(context.Context, int, bool, int, int) ([]domain.Notification, int, error) {
	return r.listItems, r.listTotal, nil
}

func (r *fakeRepo) UnreadCount(context.Context, int) (int, error) {
	return 0, nil
}

func (r *fakeRepo) MarkRead(context.Context, int, int64, time.Time) (string, error) {
	return r.link, nil
}

func (r *fakeRepo) MarkAllRead(context.Context, int, time.Time) (int64, error) {
	return 0, nil
}

func (r *fakeRepo) PruneOlderThan(context.Context, time.Time) (int64, error) {
	return 0, nil
}

func newTestHandler(link string) *Handler {
	return newTestHandlerRepo(&fakeRepo{link: link})
}

func newTestHandlerRepo(repo *fakeRepo) *Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := notificationapp.NewService(repo, notificationapp.NewBroadcaster(), logger)

	return NewHandler(svc, logger, httpx.Layout{})
}

func authed(req *http.Request) *http.Request {
	return req.WithContext(middleware.ContextWithSession(req.Context(), &accdomain.Session{UserID: 7}))
}

func TestHandler_MarkRead_HXRedirect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		link       string
		wantHeader string
	}{
		{"relative link redirects", "/tickets/9", "/tickets/9"},
		{"empty link no redirect", "", ""},
		{"protocol-relative blocked", "//evil.com", ""},
		{"absolute url blocked", "https://evil.com/x", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := newTestHandler(tt.link)
			req := httptest.NewRequest(http.MethodPost, "/notifications/3/read", http.NoBody)
			req.SetPathValue("id", "3")
			req = authed(req)
			rec := httptest.NewRecorder()

			h.markRead(rec, req)

			if rec.Code != http.StatusNoContent {
				t.Errorf("status = %d, want 204", rec.Code)
			}
			if got := rec.Header().Get("HX-Redirect"); got != tt.wantHeader {
				t.Errorf("HX-Redirect = %q, want %q", got, tt.wantHeader)
			}
		})
	}
}

func TestHandler_MarkRead_Unauthorized(t *testing.T) {
	t.Parallel()

	h := newTestHandler("/x")
	req := httptest.NewRequest(http.MethodPost, "/notifications/3/read", http.NoBody)
	req.SetPathValue("id", "3")
	rec := httptest.NewRecorder()

	h.markRead(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestHandler_MarkRead_BadID(t *testing.T) {
	t.Parallel()

	h := newTestHandler("/x")
	req := httptest.NewRequest(http.MethodPost, "/notifications/abc/read", http.NoBody)
	req.SetPathValue("id", "abc")
	req = authed(req)
	rec := httptest.NewRecorder()

	h.markRead(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandler_UnreadCount_JSON(t *testing.T) {
	t.Parallel()

	h := newTestHandler("")
	req := authed(httptest.NewRequest(http.MethodGet, "/notifications/unread-count", http.NoBody))
	rec := httptest.NewRecorder()

	h.unreadCount(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var body struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Count != 0 {
		t.Errorf("count = %d, want 0", body.Count)
	}
}

func TestHandler_UnreadCount_Unauthorized(t *testing.T) {
	t.Parallel()

	h := newTestHandler("")
	req := httptest.NewRequest(http.MethodGet, "/notifications/unread-count", http.NoBody)
	rec := httptest.NewRecorder()

	h.unreadCount(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestHandler_Inbox_RendersForAuthed(t *testing.T) {
	t.Parallel()

	h := newTestHandler("")
	req := authed(httptest.NewRequest(http.MethodGet, "/notifications?filter=unread&page=2", http.NoBody))
	rec := httptest.NewRecorder()

	h.inbox(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "Notifications") {
		t.Errorf("body missing page heading")
	}
}

func TestHandler_Inbox_Unauthorized(t *testing.T) {
	t.Parallel()

	h := newTestHandler("")
	req := httptest.NewRequest(http.MethodGet, "/notifications", http.NoBody)
	rec := httptest.NewRecorder()

	h.inbox(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestHandler_Inbox_RendersListAndPagination(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{
		listItems: []domain.Notification{{ID: 1, Title: "First"}, {ID: 2, Title: "Second"}},
		listTotal: 120,
	}
	h := newTestHandlerRepo(repo)
	req := authed(httptest.NewRequest(http.MethodGet, "/notifications", http.NoBody))
	rec := httptest.NewRecorder()

	h.inbox(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "First") || !strings.Contains(body, "Second") {
		t.Errorf("body missing notification titles")
	}
	if !strings.Contains(body, "Mark all read") {
		t.Errorf("body missing Mark all read action")
	}
	if !strings.Contains(body, "of 3") {
		t.Errorf("body missing pagination for 3 pages")
	}
}

func TestHandler_MarkAllRead_HTMXRendersMenu(t *testing.T) {
	t.Parallel()

	h := newTestHandler("")
	req := authed(httptest.NewRequest(http.MethodPost, "/notifications/read-all", http.NoBody))
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()

	h.markAllRead(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("HX-Redirect"); got != "" {
		t.Errorf("HX-Redirect = %q, want empty for htmx dropdown", got)
	}
	if !strings.Contains(rec.Body.String(), "View all") {
		t.Errorf("body is not the dropdown menu partial")
	}
}

func TestHandler_MarkAllRead_NonHTMXRedirects(t *testing.T) {
	t.Parallel()

	h := newTestHandler("")
	req := authed(httptest.NewRequest(http.MethodPost, "/notifications/read-all", http.NoBody))
	rec := httptest.NewRecorder()

	h.markAllRead(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/notifications" {
		t.Errorf("Location = %q, want /notifications", loc)
	}
}

func TestHandler_MarkAllRead_BoostedRedirects(t *testing.T) {
	t.Parallel()

	h := newTestHandler("")
	req := authed(httptest.NewRequest(http.MethodPost, "/notifications/read-all", http.NoBody))
	req.Header.Set("HX-Request", "true")
	req.Header.Set("HX-Boosted", "true")
	rec := httptest.NewRecorder()

	h.markAllRead(rec, req)

	if got := rec.Header().Get("HX-Redirect"); got != "/notifications" {
		t.Errorf("HX-Redirect = %q, want /notifications", got)
	}
}

func TestHandler_MarkAllRead_Unauthorized(t *testing.T) {
	t.Parallel()

	h := newTestHandler("")
	req := httptest.NewRequest(http.MethodPost, "/notifications/read-all", http.NoBody)
	rec := httptest.NewRecorder()

	h.markAllRead(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}
