package transport

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/a-h/templ"
	accdomain "github.com/hayakawakaki/go-racp/internal/features/account/domain"
	adminstate "github.com/hayakawakaki/go-racp/internal/features/admin/transport/state"
	itemapp "github.com/hayakawakaki/go-racp/internal/features/item/app"
	mobapp "github.com/hayakawakaki/go-racp/internal/features/mob/app"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
	"github.com/hayakawakaki/go-racp/server/config"
	admin "github.com/hayakawakaki/go-racp/themes/default/features/admin/transport"
	_ "github.com/hayakawakaki/go-racp/themes/default/platform/httpx"
)

type stubTheme struct{}

func (stubTheme) AdminLayout(layout httpx.Layout, pageTitle string, content templ.Component) templ.Component {
	return admin.AdminLayout(layout, pageTitle, content)
}

func (stubTheme) DashboardContent(state adminstate.DashboardState) templ.Component {
	return admin.DashboardContent(state)
}

func (stubTheme) DatabaseContent(state adminstate.DatabaseState) templ.Component {
	return admin.DatabaseContent(state)
}

type stubItemStatus struct {
	status itemapp.ServiceStatus
}

func (s *stubItemStatus) Status() itemapp.ServiceStatus { return s.status }

type stubMobStatus struct {
	status mobapp.ServiceStatus
}

func (s *stubMobStatus) Status() mobapp.ServiceStatus { return s.status }

type stubSession struct {
	validateFn func(context.Context, string) (*accdomain.Session, error)
}

func (s *stubSession) Validate(ctx context.Context, token string) (*accdomain.Session, error) {
	if s.validateFn != nil {
		return s.validateFn(ctx, token)
	}
	return nil, accdomain.ErrSessionNotFound
}

func (s *stubSession) Destroy(_ context.Context, _ string) error {
	return nil
}

func (s *stubSession) TTL() time.Duration { return time.Hour }

type stubUsers struct {
	getFn func(context.Context, int) (*accdomain.User, error)
}

func (s *stubUsers) GetByID(ctx context.Context, id int) (*accdomain.User, error) {
	if s.getFn != nil {
		return s.getFn(ctx, id)
	}
	return &accdomain.User{ID: id}, nil
}

func newStubSession() *stubSession  { return &stubSession{} }
func newStubUserLookup() *stubUsers { return &stubUsers{} }

func newTestHandler() *Handler {
	return NewHandler(HandlerConfig{
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		General:    config.GeneralConfig{ServerName: "Test CP", Timezone: "UTC"},
		ItemStatus: &stubItemStatus{status: itemapp.ServiceStatus{ItemsLoaded: 42, LastReload: "2026-05-18T00:00:00Z"}},
		MobStatus:  &stubMobStatus{status: mobapp.ServiceStatus{MobsLoaded: 7, LastReload: "2026-05-18T00:00:00Z"}},
		Theme:      stubTheme{},
	})
}

func TestHandler_ShowDashboard_FullPage(t *testing.T) {
	t.Parallel()
	h := newTestHandler()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", http.NoBody)
	h.showDashboard(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "<title>Test CP / Admin / Dashboard</title>") {
		t.Errorf("full page must include layout title; body:\n%s", body)
	}
	if !strings.Contains(body, `x-data="adminDashboard"`) {
		t.Errorf("full page must include dashboard content; body:\n%s", body)
	}
	if !strings.Contains(body, `id="admin-shell"`) {
		t.Errorf("full page must include admin layout shell; body:\n%s", body)
	}
}

func TestHandler_ShowDashboard_HTMXFragment(t *testing.T) {
	t.Parallel()
	h := newTestHandler()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", http.NoBody)
	req.Header.Set("HX-Request", "true")
	h.showDashboard(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `x-data="adminDashboard"`) {
		t.Errorf("HTMX fragment must include dashboard content; body:\n%s", body)
	}
	if strings.Contains(body, "<title>") {
		t.Errorf("HTMX fragment must not include layout chrome; body:\n%s", body)
	}
	if strings.Contains(body, `id="admin-shell"`) {
		t.Errorf("HTMX fragment must not include admin shell; body:\n%s", body)
	}
}

func TestHandler_RegisterRoutes_WrapsAdminRouteInRegistry(t *testing.T) {
	t.Parallel()
	h := newTestHandler()

	reg := routes.NewRegistry(
		config.AccessConfig{},
		nil,
		accdomain.NewRoleResolver(config.RolesConfig{"Moderator": 20, "Enforcer": 10, "Event": 2}),
		newStubSession(),
		newStubUserLookup(),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		false,
		true,
		httpx.Layout{},
	)
	mux := http.NewServeMux()
	h.RegisterRoutes(reg, mux)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", http.NoBody)
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("anonymous to admin must 404 (hidden); status = %d", rr.Code)
	}
}

func TestHandler_RegisterRoutes_RejectsNonGet(t *testing.T) {
	t.Parallel()
	h := newTestHandler()

	reg := routes.NewRegistry(
		config.AccessConfig{},
		nil,
		accdomain.NewRoleResolver(config.RolesConfig{"Moderator": 20, "Enforcer": 10, "Event": 2}),
		newStubSession(),
		newStubUserLookup(),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		false,
		true,
		httpx.Layout{},
	)
	mux := http.NewServeMux()
	h.RegisterRoutes(reg, mux)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin", http.NoBody)
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}
