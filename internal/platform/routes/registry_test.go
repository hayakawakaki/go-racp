package routes

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	accdomain "github.com/hayakawakaki/go-racp/internal/features/account/domain"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
	"github.com/hayakawakaki/go-racp/server/config"
	_ "github.com/hayakawakaki/go-racp/themes/default/platform/httpx"
)

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

func newRegistry(t *testing.T, cfg config.AccessConfig) (*Registry, *bytes.Buffer) {
	t.Helper()
	buf := &bytes.Buffer{}
	resolver := accdomain.NewRoleResolver(config.RolesConfig{"Moderator": 20, "Enforcer": 10, "Event": 2})
	logger := slog.New(slog.NewTextHandler(buf, nil))
	return NewRegistry(cfg, nil, resolver, &stubSession{}, &stubUsers{}, logger, false, true, httpx.Layout{}, nil), buf
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

func TestRegistry_Wrap_MemberMeansAuthenticated(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	reg, _ := newRegistry(t, config.AccessConfig{
		"Account": config.ActionRoles{"View": config.Entry{Roles: config.RoleList{"Member"}}},
	})

	reg.Wrap(mux, "Account.View", "GET /account", okHandler())

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/account", http.NoBody)
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Errorf("anonymous on Member route must redirect; status = %d, want 303", rr.Code)
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
	resolver := accdomain.NewRoleResolver(config.RolesConfig{"Moderator": 20})
	logger := slog.New(slog.NewTextHandler(buf, nil))
	sess := &stubSession{
		validateFn: func(context.Context, string) (*accdomain.Session, error) {
			return &accdomain.Session{UserID: 1}, nil
		},
	}
	users := &stubUsers{
		getFn: func(context.Context, int) (*accdomain.User, error) {
			return &accdomain.User{ID: 1, State: 0, UnbanTime: time.Now().Add(time.Hour)}, nil
		},
	}
	reg := NewRegistry(config.AccessConfig{}, nil, resolver, sess, users, logger, false, true, httpx.Layout{}, nil)

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
	resolver := accdomain.NewRoleResolver(config.RolesConfig{"Moderator": 20})
	logger := slog.New(slog.NewTextHandler(buf, nil))
	sess := &stubSession{
		validateFn: func(context.Context, string) (*accdomain.Session, error) {
			return &accdomain.Session{UserID: 1}, nil
		},
	}
	users := &stubUsers{
		getFn: func(context.Context, int) (*accdomain.User, error) {
			return &accdomain.User{ID: 1, State: 0, UnbanTime: time.Now().Add(time.Hour)}, nil
		},
	}
	cfg := config.AccessConfig{
		"Account": config.ActionRoles{
			"ChangePassword": config.Entry{Roles: config.RoleList{"Verified"}, Requires: []string{"Unrestricted"}},
		},
	}
	reg := NewRegistry(cfg, nil, resolver, sess, users, logger, false, true, httpx.Layout{}, nil)

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
	resolver := accdomain.NewRoleResolver(config.RolesConfig{"Moderator": 20})
	logger := slog.New(slog.NewTextHandler(buf, nil))
	sess := &stubSession{
		validateFn: func(context.Context, string) (*accdomain.Session, error) {
			return &accdomain.Session{UserID: 1}, nil
		},
	}
	users := &stubUsers{
		getFn: func(context.Context, int) (*accdomain.User, error) {
			return &accdomain.User{ID: 1, State: 0, UnbanTime: time.Now().Add(time.Hour)}, nil
		},
	}
	cfg := config.AccessConfig{
		"Account": config.ActionRoles{
			"View": config.Entry{Roles: config.RoleList{"Member"}},
		},
	}
	reg := NewRegistry(cfg, nil, resolver, sess, users, logger, false, true, httpx.Layout{}, nil)

	reg.Wrap(mux, "Account.View", "GET /account", okHandler())

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/account", http.NoBody)
	req.AddCookie(&http.Cookie{Name: "racp_session", Value: "valid"})
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("temp-banned user on non-Unrestricted route must pass through; got %d body=%q", rr.Code, rr.Body.String())
	}
}

func TestRegistry_Wrap_MemberRouteAdmitsUnverified(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	buf := &bytes.Buffer{}
	resolver := accdomain.NewRoleResolver(config.RolesConfig{"Moderator": 20})
	logger := slog.New(slog.NewTextHandler(buf, nil))
	sess := &stubSession{
		validateFn: func(context.Context, string) (*accdomain.Session, error) {
			return &accdomain.Session{UserID: 1}, nil
		},
	}
	users := &stubUsers{
		getFn: func(context.Context, int) (*accdomain.User, error) {
			return &accdomain.User{ID: 1, GroupID: 0, State: 1}, nil
		},
	}
	cfg := config.AccessConfig{
		"Notification": config.ActionRoles{
			"View": config.Entry{Roles: config.RoleList{"Member"}},
		},
	}
	reg := NewRegistry(cfg, nil, resolver, sess, users, logger, false, true, httpx.Layout{}, nil)

	reg.Wrap(mux, "Notification.View", "GET /notifications", okHandler())

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/notifications", http.NoBody)
	req.AddCookie(&http.Cookie{Name: "racp_session", Value: "valid"})
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("unverified user on Member route must pass through; got %d body=%q", rr.Code, rr.Body.String())
	}
}

func TestRegistry_Wrap_VerifiedRouteRedirectsUnverified(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	buf := &bytes.Buffer{}
	resolver := accdomain.NewRoleResolver(config.RolesConfig{"Moderator": 20})
	logger := slog.New(slog.NewTextHandler(buf, nil))
	sess := &stubSession{
		validateFn: func(context.Context, string) (*accdomain.Session, error) {
			return &accdomain.Session{UserID: 1}, nil
		},
	}
	users := &stubUsers{
		getFn: func(context.Context, int) (*accdomain.User, error) {
			return &accdomain.User{ID: 1, GroupID: 0, State: 1}, nil
		},
	}
	cfg := config.AccessConfig{
		"Account": config.ActionRoles{
			"ChangeEmail": config.Entry{Roles: config.RoleList{"Verified"}, Requires: []string{"Unrestricted"}},
		},
	}
	reg := NewRegistry(cfg, nil, resolver, sess, users, logger, false, true, httpx.Layout{}, nil)

	reg.Wrap(mux, "Account.ChangeEmail", "POST /account/email", okHandler())

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/account/email", http.NoBody)
	req.AddCookie(&http.Cookie{Name: "racp_session", Value: "valid"})
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("unverified user on Verified route must redirect; got %d", rr.Code)
	}
	if got := rr.Header().Get("Location"); got != "/verify-account" {
		t.Errorf("Location = %q, want /verify-account", got)
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

func TestRegistry_Finalize_WarnsButDoesNotPanicOnDeadThemeEntries(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()

	cfg := config.AccessConfig{
		"ThemePages": config.ActionRoles{
			"Orphaned": config.Entry{Roles: config.RoleList{"Public"}},
		},
	}

	reg, buf := newRegistry(t, cfg)
	_ = mux

	reg.Finalize()

	if !strings.Contains(buf.String(), "ThemePages.Orphaned") {
		t.Errorf("expected warning to mention ThemePages.Orphaned, got log: %s", buf.String())
	}

	if !strings.Contains(buf.String(), "dead entries") {
		t.Errorf("expected warning to mention dead entries, got log: %s", buf.String())
	}
}

func TestRegistry_Finalize_PanicsOnDeadNonThemeEntryEvenWhenThemeAlsoDead(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()

	cfg := config.AccessConfig{
		"News":       config.ActionRoles{"Edit": config.Entry{Roles: config.RoleList{"Moderator"}}},
		"ThemePages": config.ActionRoles{"Orphaned": config.Entry{Roles: config.RoleList{"Public"}}},
	}

	reg, _ := newRegistry(t, cfg)
	_ = mux

	defer func() {
		v := recover()
		if v == nil {
			t.Fatalf("Finalize did not panic when slice tag was also dead")
		}
		msg, ok := v.(error)
		if !ok || !strings.Contains(msg.Error(), "News.Edit") {
			t.Errorf("panic = %v, want mention of News.Edit", v)
		}

		if ok && strings.Contains(msg.Error(), "ThemePages.Orphaned") {
			t.Errorf("panic message should not include ThemePages.* (those are warnings): %v", v)
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

func TestRegistry_Wrap_SoleAdminEntryIs404ForNonAdmin(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	buf := &bytes.Buffer{}
	resolver := accdomain.NewRoleResolver(config.RolesConfig{"Moderator": 20})
	logger := slog.New(slog.NewTextHandler(buf, nil))
	sess := &stubSession{
		validateFn: func(context.Context, string) (*accdomain.Session, error) {
			return &accdomain.Session{UserID: 1}, nil
		},
	}
	users := &stubUsers{
		getFn: func(context.Context, int) (*accdomain.User, error) {
			return &accdomain.User{ID: 1, GroupID: 20, State: 0}, nil
		},
	}
	cfg := config.AccessConfig{
		"Users": config.ActionRoles{
			"List": config.Entry{Roles: config.RoleList{"Admin"}},
		},
	}
	reg := NewRegistry(cfg, nil, resolver, sess, users, logger, false, true, httpx.Layout{}, nil)

	reg.Wrap(mux, "Users.List", "GET /users", okHandler())

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users", http.NoBody)
	req.AddCookie(&http.Cookie{Name: "racp_session", Value: "valid"})
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("non-admin hitting sole-Admin route must 404 (hidden); got %d", rr.Code)
	}
}

func TestRegistry_WrapHidden(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		cookie    bool
		userGroup int
		wantCode  int
	}{
		{name: "anonymous gets 404", cookie: false, wantCode: http.StatusNotFound},
		{name: "wrong role gets 404", cookie: true, userGroup: 20, wantCode: http.StatusNotFound},
		{name: "enforcer passes", cookie: true, userGroup: 10, wantCode: http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mux := http.NewServeMux()
			buf := &bytes.Buffer{}
			resolver := accdomain.NewRoleResolver(config.RolesConfig{"Enforcer": 10, "Moderator": 20})
			logger := slog.New(slog.NewTextHandler(buf, nil))
			sess := &stubSession{
				validateFn: func(context.Context, string) (*accdomain.Session, error) {
					return &accdomain.Session{UserID: 1}, nil
				},
			}
			users := &stubUsers{
				getFn: func(_ context.Context, id int) (*accdomain.User, error) {
					return &accdomain.User{ID: id, GroupID: tt.userGroup, State: 0}, nil
				},
			}
			cfg := config.AccessConfig{
				"Users": config.ActionRoles{
					"View": config.Entry{Roles: config.RoleList{"Enforcer"}},
				},
			}
			reg := NewRegistry(cfg, nil, resolver, sess, users, logger, false, true, httpx.Layout{}, nil)

			reg.WrapHidden(mux, "Users.View", "GET /users/{id}", okHandler())

			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/users/7", http.NoBody)
			if tt.cookie {
				req.AddCookie(&http.Cookie{Name: "racp_session", Value: "valid"})
			}
			mux.ServeHTTP(rr, req)

			if rr.Code != tt.wantCode {
				t.Errorf("status = %d, want %d", rr.Code, tt.wantCode)
			}
		})
	}
}

func TestRegistry_RoutesSnapshot_EmptyByDefault(t *testing.T) {
	t.Parallel()

	reg, _ := newRegistry(t, config.AccessConfig{})

	got := reg.RoutesSnapshot()
	if len(got) != 0 {
		t.Errorf("fresh registry should have empty snapshot, got %d entries: %+v", len(got), got)
	}
}

func TestRegistry_RoutesSnapshot_RecordsEveryWrapCall(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()

	cfg := config.AccessConfig{
		"Home":       config.ActionRoles{"View": config.Entry{Roles: config.RoleList{"Public"}}},
		"Account":    config.ActionRoles{"View": config.Entry{Roles: config.RoleList{"Member"}}},
		"ThemePages": config.ActionRoles{"Rates": config.Entry{Roles: config.RoleList{"Public"}}},
	}

	reg, _ := newRegistry(t, cfg)

	reg.Wrap(mux, "Home.View", "GET /", okHandler())
	reg.Wrap(mux, "Account.View", "GET /account", okHandler())
	reg.Wrap(mux, "ThemePages.Rates", "GET /rates", okHandler())
	reg.Wrap(mux, "Admin.Dashboard", "GET /admin", okHandler())

	got := reg.RoutesSnapshot()

	want := []RouteInfo{
		{Tag: "Home.View", Pattern: "GET /"},
		{Tag: "Account.View", Pattern: "GET /account"},
		{Tag: "ThemePages.Rates", Pattern: "GET /rates"},
		{Tag: "Admin.Dashboard", Pattern: "GET /admin"},
	}

	if !slices.Equal(got, want) {
		t.Errorf("RoutesSnapshot = %+v\nwant insertion order %+v", got, want)
	}
}

func TestRegistry_RoutesSnapshot_ReturnsIndependentCopy(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()

	cfg := config.AccessConfig{
		"Home": config.ActionRoles{"View": config.Entry{Roles: config.RoleList{"Public"}}},
	}

	reg, _ := newRegistry(t, cfg)
	reg.Wrap(mux, "Home.View", "GET /", okHandler())

	first := reg.RoutesSnapshot()
	first[0].Tag = "Mutated"

	second := reg.RoutesSnapshot()
	if second[0].Tag != "Home.View" {
		t.Errorf("mutating returned snapshot leaked into registry state: got Tag=%q, want Home.View", second[0].Tag)
	}
}
