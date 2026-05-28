package moderation

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/a-h/templ"
	app "github.com/hayakawakaki/go-racp/internal/features/account/app/moderation"
	accdomain "github.com/hayakawakaki/go-racp/internal/features/account/domain"
	"github.com/hayakawakaki/go-racp/internal/features/account/transport/middleware"
	accountmoderationstate "github.com/hayakawakaki/go-racp/internal/features/account/transport/moderation/state"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
	"github.com/hayakawakaki/go-racp/server/config"
	accountmoderation "github.com/hayakawakaki/go-racp/themes/default/features/account/transport/moderation"
	_ "github.com/hayakawakaki/go-racp/themes/default/platform/httpx"
)

func middlewareCtxWithActor(ctx context.Context, userID int) context.Context {
	return middleware.ContextWithSnapshot(ctx, &middleware.AccountSnapshot{UserID: userID})
}

type stubService struct {
	getFn   func(context.Context, int) (app.UserDetail, error)
	banFn   func(context.Context, app.BanCommand) (app.UserDetail, error)
	unbanFn func(context.Context, app.UnbanCommand) (app.UserDetail, error)
	roleFn  func(context.Context, app.SetRoleCommand) (app.UserDetail, error)
	allowed map[int]string
}

func (s *stubService) Get(ctx context.Context, id int) (app.UserDetail, error) {
	if s.getFn != nil {
		return s.getFn(ctx, id)
	}
	return app.UserDetail{}, accdomain.ErrUserNotFound
}
func (s *stubService) Ban(ctx context.Context, cmd app.BanCommand) (app.UserDetail, error) {
	if s.banFn != nil {
		return s.banFn(ctx, cmd)
	}
	return app.UserDetail{}, nil
}
func (s *stubService) Unban(ctx context.Context, cmd app.UnbanCommand) (app.UserDetail, error) {
	if s.unbanFn != nil {
		return s.unbanFn(ctx, cmd)
	}
	return app.UserDetail{}, nil
}
func (s *stubService) SetRole(ctx context.Context, cmd app.SetRoleCommand) (app.UserDetail, error) {
	if s.roleFn != nil {
		return s.roleFn(ctx, cmd)
	}
	return app.UserDetail{}, nil
}
func (s *stubService) AllowedRoles() map[int]string {
	if s.allowed == nil {
		return map[int]string{0: "Player"}
	}
	return s.allowed
}

type stubTheme struct{}

func (stubTheme) UsersDetailPage(layout httpx.Layout, username string, state accountmoderationstate.DetailState) templ.Component {
	return accountmoderation.UsersDetailPage(layout, username, state)
}
func (stubTheme) UsersDetailContent(state accountmoderationstate.DetailState) templ.Component {
	return accountmoderation.UsersDetailContent(state)
}
func (stubTheme) UsersNotFoundPage(layout httpx.Layout, id string) templ.Component {
	return accountmoderation.UsersNotFoundPage(layout, id)
}
func (stubTheme) UsersActionError(message string) templ.Component {
	return accountmoderation.UsersActionError(message)
}

func TestHandler_ShowDetail_RendersUsernameAndChars(t *testing.T) {
	t.Parallel()
	svc := &stubService{
		getFn: func(_ context.Context, _ int) (app.UserDetail, error) {
			return app.UserDetail{
				User:       &accdomain.User{ID: 7, Username: "kaki", Email: "k@example.com"},
				Characters: []accdomain.Character{{ID: 1, Name: "Aurora", Class: 1, BaseLevel: 50}},
			}, nil
		},
		allowed: map[int]string{0: "Player", 20: "Moderator"},
	}
	h := NewHandler(svc, HandlerConfig{
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		General: config.GeneralConfig{ServerName: "Test CP", Timezone: "UTC"},
		Theme:   stubTheme{},
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users/7", http.NoBody)
	req.SetPathValue("id", "7")
	h.showDetail(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "kaki") || !strings.Contains(body, "Aurora") {
		t.Errorf("body missing fields:\n%s", body)
	}
}

func TestHandler_ShowDetail_NotFound(t *testing.T) {
	t.Parallel()
	svc := &stubService{
		getFn: func(_ context.Context, _ int) (app.UserDetail, error) {
			return app.UserDetail{}, accdomain.ErrUserNotFound
		},
	}
	h := NewHandler(svc, HandlerConfig{
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		General: config.GeneralConfig{ServerName: "Test CP", Timezone: "UTC"},
		Theme:   stubTheme{},
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users/9999", http.NoBody)
	req.SetPathValue("id", "9999")
	h.showDetail(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

func TestHandler_DoBan_SwapsDetail(t *testing.T) {
	t.Parallel()
	called := false
	svc := &stubService{
		banFn: func(_ context.Context, cmd app.BanCommand) (app.UserDetail, error) {
			called = true
			if !cmd.Permanent || cmd.Reason != "spam" {
				t.Errorf("unexpected cmd: %+v", cmd)
			}
			return app.UserDetail{User: &accdomain.User{ID: 7, Username: "kaki", State: 5}}, nil
		},
	}
	h := NewHandler(svc, HandlerConfig{
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		General: config.GeneralConfig{ServerName: "Test CP", Timezone: "UTC"},
		Theme:   stubTheme{},
	})

	rr := httptest.NewRecorder()
	body := strings.NewReader("permanent=on&reason=spam")
	req := httptest.NewRequest(http.MethodPost, "/users/7/ban", body)
	req.SetPathValue("id", "7")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(middlewareCtxWithActor(req.Context(), 1))
	h.doBan(rr, req)

	if !called {
		t.Errorf("svc.Ban not called")
	}
	if !strings.Contains(rr.Body.String(), `id="user-detail"`) {
		t.Errorf("response should swap full detail; body=%s", rr.Body.String())
	}
}

func TestHandler_DoBan_ValidationError(t *testing.T) {
	t.Parallel()
	svc := &stubService{
		banFn: func(_ context.Context, _ app.BanCommand) (app.UserDetail, error) {
			return app.UserDetail{}, accdomain.ErrEmptyReason
		},
	}
	h := NewHandler(svc, HandlerConfig{
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		General: config.GeneralConfig{ServerName: "Test CP", Timezone: "UTC"},
		Theme:   stubTheme{},
	})

	rr := httptest.NewRecorder()
	body := strings.NewReader("permanent=on")
	req := httptest.NewRequest(http.MethodPost, "/users/7/ban", body)
	req.SetPathValue("id", "7")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(middlewareCtxWithActor(req.Context(), 1))
	h.doBan(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Reason is required") {
		t.Errorf("body should mention validation; got %s", rr.Body.String())
	}
}

func TestHandler_DoUnban_Calls(t *testing.T) {
	t.Parallel()
	called := false
	svc := &stubService{
		unbanFn: func(_ context.Context, cmd app.UnbanCommand) (app.UserDetail, error) {
			called = true
			if cmd.TargetUserID != 7 {
				t.Errorf("target = %d", cmd.TargetUserID)
			}
			return app.UserDetail{User: &accdomain.User{ID: 7, Username: "kaki"}}, nil
		},
	}
	h := NewHandler(svc, HandlerConfig{
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		General: config.GeneralConfig{ServerName: "Test CP", Timezone: "UTC"},
		Theme:   stubTheme{},
	})

	rr := httptest.NewRecorder()
	body := strings.NewReader("reason=appeal")
	req := httptest.NewRequest(http.MethodPost, "/users/7/unban", body)
	req.SetPathValue("id", "7")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(middlewareCtxWithActor(req.Context(), 1))
	h.doUnban(rr, req)

	if !called {
		t.Errorf("Unban not called")
	}
	if !strings.Contains(rr.Body.String(), `id="user-detail"`) {
		t.Errorf("expected detail swap")
	}
}

func TestHandler_DoSetRole_Calls(t *testing.T) {
	t.Parallel()
	called := false
	svc := &stubService{
		roleFn: func(_ context.Context, cmd app.SetRoleCommand) (app.UserDetail, error) {
			called = true
			if cmd.NewGroupID != 20 {
				t.Errorf("group = %d", cmd.NewGroupID)
			}
			return app.UserDetail{User: &accdomain.User{ID: 7, Username: "kaki", GroupID: 20}}, nil
		},
		allowed: map[int]string{0: "Player", 20: "Moderator"},
	}
	h := NewHandler(svc, HandlerConfig{
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		General: config.GeneralConfig{ServerName: "Test CP", Timezone: "UTC"},
		Theme:   stubTheme{},
	})

	rr := httptest.NewRecorder()
	body := strings.NewReader("group_id=20&reason=promote")
	req := httptest.NewRequest(http.MethodPost, "/users/7/role", body)
	req.SetPathValue("id", "7")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(middlewareCtxWithActor(req.Context(), 1))
	h.doSetRole(rr, req)

	if !called {
		t.Errorf("SetRole not called")
	}
	if !strings.Contains(rr.Body.String(), `id="user-detail"`) {
		t.Errorf("expected detail swap")
	}
}

func TestBuildRoleOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		allowed map[int]string
		want    []accountmoderationstate.RoleOption
	}{
		{
			name:    "empty input returns empty slice",
			allowed: map[int]string{},
			want:    []accountmoderationstate.RoleOption{},
		},
		{
			name:    "single role",
			allowed: map[int]string{0: "Player"},
			want:    []accountmoderationstate.RoleOption{{GroupID: 0, Name: "Player"}},
		},
		{
			name:    "sorted ascending by group_id",
			allowed: map[int]string{20: "Moderator", 0: "Player", 10: "Enforcer", 2: "Event"},
			want: []accountmoderationstate.RoleOption{
				{GroupID: 0, Name: "Player"},
				{GroupID: 2, Name: "Event"},
				{GroupID: 10, Name: "Enforcer"},
				{GroupID: 20, Name: "Moderator"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := accountmoderationstate.BuildRoleOptions(tt.allowed)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d (%+v)", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestRoleNameFor(t *testing.T) {
	t.Parallel()

	state := accountmoderationstate.DetailState{
		AllowedRoles: []accountmoderationstate.RoleOption{
			{GroupID: 0, Name: "Player"},
			{GroupID: 20, Name: "Moderator"},
		},
	}

	tests := []struct {
		name    string
		want    string
		groupID int
	}{
		{name: "known player role", groupID: 0, want: "Player"},
		{name: "known moderator role", groupID: 20, want: "Moderator"},
		{name: "admin group resolves to Admin", groupID: accdomain.RoleAdmin.GroupID, want: "Admin"},
		{name: "unknown group falls back", groupID: 77, want: "group_77"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := accountmoderationstate.RoleNameFor(state, tt.groupID); got != tt.want {
				t.Errorf("RoleNameFor(%d) = %q, want %q", tt.groupID, got, tt.want)
			}
		})
	}
}

func middlewareCtxWithActorGroup(ctx context.Context, userID, groupID int) context.Context {
	return middleware.ContextWithSnapshot(ctx, &middleware.AccountSnapshot{UserID: userID, GroupID: groupID})
}

func TestHandler_DoBan_ThreadsActorIsAdminFromSnapshot(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		actorGroupID int
		wantIsAdmin  bool
	}{
		{"admin actor", accdomain.RoleAdmin.GroupID, true},
		{"moderator actor", 20, false},
		{"player actor", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var gotIsAdmin bool
			svc := &stubService{
				banFn: func(_ context.Context, cmd app.BanCommand) (app.UserDetail, error) {
					gotIsAdmin = cmd.ActorIsAdmin
					return app.UserDetail{User: &accdomain.User{ID: 7, Username: "kaki"}}, nil
				},
			}
			h := NewHandler(svc, HandlerConfig{
				Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
				General: config.GeneralConfig{ServerName: "Test CP", Timezone: "UTC"},
				Theme:   stubTheme{},
			})

			rr := httptest.NewRecorder()
			body := strings.NewReader("permanent=on&reason=spam")
			req := httptest.NewRequest(http.MethodPost, "/users/7/ban", body)
			req.SetPathValue("id", "7")
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req = req.WithContext(middlewareCtxWithActorGroup(req.Context(), 1, tt.actorGroupID))
			h.doBan(rr, req)

			if gotIsAdmin != tt.wantIsAdmin {
				t.Errorf("ActorIsAdmin = %v, want %v", gotIsAdmin, tt.wantIsAdmin)
			}
		})
	}
}
