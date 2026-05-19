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

func TestRegistry_Wrap_PublicSentinelMountsUnwrapped(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	reg, _ := newRegistry(t, config.AccessConfig{
		"Account": config.ActionRoles{"Login": config.Entry{Roles: config.RoleList{"Public"}}},
	})

	reg.Wrap(mux, "Account.Login", "GET /login", okHandler())

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/login", http.NoBody)
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Public sentinel must mount handler unwrapped; status = %d, want 200", rr.Code)
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

func TestRegistry_Wrap_EmptyRolesPanics(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	reg, _ := newRegistry(t, config.AccessConfig{
		"News": config.ActionRoles{"View": config.Entry{Roles: config.RoleList{}, Requires: []string{"Unrestricted"}}},
	})

	defer func() {
		v := recover()
		if v == nil {
			t.Fatalf("expected panic for configured entry with empty Roles")
		}
		msg, ok := v.(error)
		if !ok || !strings.Contains(msg.Error(), "News") {
			t.Errorf("panic = %v, want mention of News.View", v)
		}
	}()
	reg.Wrap(mux, "News.View", "GET /news", okHandler())
}

func TestRegistry_Wrap_AdminTagBlocksTempBanned(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	buf := &bytes.Buffer{}
	resolver := domain.NewRoleResolver(config.RolesConfig{"Moderator": 20})
	logger := slog.New(slog.NewTextHandler(buf, nil))
	sess := &stubSession{
		validateFn: func(context.Context, string) (*domain.Session, error) {
			return &domain.Session{UserID: 1}, nil
		},
	}
	users := &stubUsers{
		getFn: func(context.Context, int) (*domain.User, error) {
			return &domain.User{ID: 1, State: 0, UnbanTime: time.Now().Add(time.Hour)}, nil
		},
	}
	reg := NewRegistry(config.AccessConfig{}, resolver, sess, users, logger, false, true, httpx.Layout{})

	reg.Wrap(mux, "Admin.Dashboard", "GET /admin", okHandler())

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", http.NoBody)
	req.AddCookie(&http.Cookie{Name: "racp_session", Value: "valid"})
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("temp-banned admin hitting Admin route must 404 (hidden + FullAccess implied); got %d", rr.Code)
	}
}

func TestRegistry_Wrap_UnrestrictedRouteSoftBlocksTempBanned(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	buf := &bytes.Buffer{}
	resolver := domain.NewRoleResolver(config.RolesConfig{"Moderator": 20})
	logger := slog.New(slog.NewTextHandler(buf, nil))
	sess := &stubSession{
		validateFn: func(context.Context, string) (*domain.Session, error) {
			return &domain.Session{UserID: 1}, nil
		},
	}
	users := &stubUsers{
		getFn: func(context.Context, int) (*domain.User, error) {
			return &domain.User{ID: 1, State: 0, UnbanTime: time.Now().Add(time.Hour)}, nil
		},
	}
	cfg := config.AccessConfig{
		"Account": config.ActionRoles{
			"ChangePassword": config.Entry{Roles: config.RoleList{"*"}, Requires: []string{"Unrestricted"}},
		},
	}
	reg := NewRegistry(cfg, resolver, sess, users, logger, false, true, httpx.Layout{})

	reg.Wrap(mux, "Account.ChangePassword", "POST /account/password", okHandler())

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/account/password", http.NoBody)
	req.AddCookie(&http.Cookie{Name: "racp_session", Value: "valid"})
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("temp-banned user on Unrestricted route must redirect; got %d", rr.Code)
	}
	if got := rr.Header().Get("Location"); got != "/account?notice=ban_blocked" {
		t.Errorf("Location = %q, want /account?notice=ban_blocked", got)
	}
}

func TestRegistry_Wrap_NonUnrestrictedRouteAllowsTempBanned(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	buf := &bytes.Buffer{}
	resolver := domain.NewRoleResolver(config.RolesConfig{"Moderator": 20})
	logger := slog.New(slog.NewTextHandler(buf, nil))
	sess := &stubSession{
		validateFn: func(context.Context, string) (*domain.Session, error) {
			return &domain.Session{UserID: 1}, nil
		},
	}
	users := &stubUsers{
		getFn: func(context.Context, int) (*domain.User, error) {
			return &domain.User{ID: 1, State: 0, UnbanTime: time.Now().Add(time.Hour)}, nil
		},
	}
	cfg := config.AccessConfig{
		"Account": config.ActionRoles{
			"View": config.Entry{Roles: config.RoleList{"*"}},
		},
	}
	reg := NewRegistry(cfg, resolver, sess, users, logger, false, true, httpx.Layout{})

	reg.Wrap(mux, "Account.View", "GET /account", okHandler())

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/account", http.NoBody)
	req.AddCookie(&http.Cookie{Name: "racp_session", Value: "valid"})
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("temp-banned user on non-Unrestricted route must pass through; got %d body=%q", rr.Code, rr.Body.String())
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

func TestRegistry_Finalize_PanicsOnUngatedTag(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	reg, _ := newRegistry(t, config.AccessConfig{})
	reg.Wrap(mux, "Forum.View", "GET /forum", okHandler())

	defer func() {
		v := recover()
		if v == nil {
			t.Fatalf("Finalize did not panic for ungated tag")
		}
		msg, ok := v.(error)
		if !ok || !strings.Contains(msg.Error(), "Forum.View") {
			t.Errorf("panic = %v, want mention of Forum.View", v)
		}
	}()
	reg.Finalize()
}

func TestRegistry_Finalize_SilentForAdminTags(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	reg, _ := newRegistry(t, config.AccessConfig{})
	reg.Wrap(mux, "Admin.Dashboard", "GET /admin", okHandler())
	reg.Finalize()
}
