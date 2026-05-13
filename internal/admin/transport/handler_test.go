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

	"github.com/hayakawakaki/go-racp/internal/account/domain"
	"github.com/hayakawakaki/go-racp/internal/httpx"
	"github.com/hayakawakaki/go-racp/internal/routes"
	"github.com/hayakawakaki/go-racp/server/config"
)

type stubSession struct {
	validateFn func(context.Context, string) (*domain.Session, error)
}

func (s *stubSession) Validate(ctx context.Context, token string) (*domain.Session, error) {
	if s.validateFn != nil {
		return s.validateFn(ctx, token)
	}
	return nil, domain.ErrSessionNotFound
}

func (s *stubSession) TTL() time.Duration { return time.Hour }

type stubUsers struct {
	getFn func(context.Context, int) (*domain.User, error)
}

func (s *stubUsers) GetByID(ctx context.Context, id int) (*domain.User, error) {
	if s.getFn != nil {
		return s.getFn(ctx, id)
	}
	return &domain.User{ID: id}, nil
}

func newStubSession() *stubSession  { return &stubSession{} }
func newStubUserLookup() *stubUsers { return &stubUsers{} }

func newTestHandler() *Handler {
	return NewHandler(HandlerConfig{
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		General: config.GeneralConfig{ServerName: "Test CP", Timezone: "UTC"},
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
	if !strings.Contains(body, "Welcome to the admin panel") {
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
	if !strings.Contains(body, "Welcome to the admin panel") {
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
		domain.NewRoleResolver(config.RolesConfig{"Moderator": 20, "Enforcer": 10, "Event": 2}),
		newStubSession(),
		newStubUserLookup(),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		false,
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
		domain.NewRoleResolver(config.RolesConfig{"Moderator": 20, "Enforcer": 10, "Event": 2}),
		newStubSession(),
		newStubUserLookup(),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		false,
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
