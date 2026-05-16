package routes

import (
	"bytes"
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

func (s *stubSession) Destroy(_ context.Context, _ string) error {
	return nil
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

func newRegistry(t *testing.T, cfg config.AccessConfig) (*Registry, *bytes.Buffer) {
	t.Helper()
	buf := &bytes.Buffer{}
	resolver := domain.NewRoleResolver(config.RolesConfig{"Moderator": 20, "Enforcer": 10, "Event": 2})
	logger := slog.New(slog.NewTextHandler(buf, nil))
	return NewRegistry(cfg, resolver, &stubSession{}, &stubUsers{}, logger, false, true, httpx.Layout{}), buf
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	})
}

func TestRegistry_Public_PassesThroughUnwrapped(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	reg, _ := newRegistry(t, config.AccessConfig{})

	reg.Public(mux, "GET /login", okHandler())

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/login", http.NoBody)
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
}

func TestRegistry_Wrap_AdminTagIsHardcoded(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	reg, _ := newRegistry(t, config.AccessConfig{})

	reg.Wrap(mux, "Admin.Dashboard", "GET /admin", okHandler())

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", http.NoBody)
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("admin without session must 404, got %d", rr.Code)
	}
}

func TestRegistry_Wrap_TaggedWithoutConfigRequiresAuth(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	reg, _ := newRegistry(t, config.AccessConfig{})

	reg.Wrap(mux, "Forum.View", "GET /forum", okHandler())

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/forum", http.NoBody)
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Errorf("ungated tagged route requires auth; status = %d, want 303", rr.Code)
	}
}

func TestRegistry_Wrap_TaggedWithConfigGatesRequest(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	reg, _ := newRegistry(t, config.AccessConfig{
		"News": config.ActionRoles{"View": config.Entry{Roles: config.RoleList{"Moderator"}}},
	})

	reg.Wrap(mux, "News.View", "GET /news", okHandler())

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/news", http.NoBody)
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Errorf("anonymous gated route must redirect; status = %d, want 303", rr.Code)
	}
}

func TestRegistry_Wrap_StarMeansAuthenticated(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	reg, _ := newRegistry(t, config.AccessConfig{
		"Account": config.ActionRoles{"View": config.Entry{Roles: config.RoleList{"*"}}},
	})

	reg.Wrap(mux, "Account.View", "GET /account", okHandler())

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/account", http.NoBody)
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Errorf("anonymous on * route must redirect; status = %d, want 303", rr.Code)
	}
}

func TestRegistry_Wrap_RejectsMalformedTag(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		tag  string
	}{
		{name: "no dot", tag: "Dashboard"},
		{name: "three segments", tag: "News.View.Detail"},
		{name: "empty group", tag: ".View"},
		{name: "empty action", tag: "News."},
		{name: "only dot", tag: "."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mux := http.NewServeMux()
			reg, _ := newRegistry(t, config.AccessConfig{})
			defer func() {
				if recover() == nil {
					t.Errorf("Wrap(%q) did not panic", tt.tag)
				}
			}()
			reg.Wrap(mux, tt.tag, "GET /x", okHandler())
		})
	}
}

func TestRegistry_Finalize_PanicsOnDeadConfig(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	reg, _ := newRegistry(t, config.AccessConfig{
		"News": config.ActionRoles{"Edit": config.Entry{Roles: config.RoleList{"Moderator"}}},
	})
	reg.Wrap(mux, "News.View", "GET /news", okHandler())

	defer func() {
		v := recover()
		if v == nil {
			t.Fatalf("Finalize did not panic for dead config entry")
		}
		msg, ok := v.(error)
		if !ok || !strings.Contains(msg.Error(), "News.Edit") {
			t.Errorf("panic = %v, want mention of News.Edit", v)
		}
	}()
	reg.Finalize()
}

func TestRegistry_Finalize_LogsUngatedAudit(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	reg, buf := newRegistry(t, config.AccessConfig{})
	reg.Wrap(mux, "Forum.View", "GET /forum", okHandler())
	reg.Finalize()
	if !strings.Contains(buf.String(), "ungated") {
		t.Errorf("expected audit log entry for ungated route; got %q", buf.String())
	}
	if !strings.Contains(buf.String(), "Forum.View") {
		t.Errorf("audit log missing tag name; got %q", buf.String())
	}
}

func TestRegistry_Finalize_SilentForAdminTagsAndPublic(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	reg, buf := newRegistry(t, config.AccessConfig{})
	reg.Wrap(mux, "Admin.Dashboard", "GET /admin", okHandler())
	reg.Public(mux, "GET /login", okHandler())
	reg.Finalize()
	if strings.Contains(buf.String(), "Admin.Dashboard") {
		t.Errorf("Admin tags must not appear in ungated audit; got %q", buf.String())
	}
	if strings.Contains(buf.String(), "GET /login") {
		t.Errorf("Public routes must not appear in ungated audit; got %q", buf.String())
	}
}
