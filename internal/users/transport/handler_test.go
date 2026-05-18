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
	"github.com/hayakawakaki/go-racp/internal/account/transport/middleware"
	"github.com/hayakawakaki/go-racp/internal/httpx"
	"github.com/hayakawakaki/go-racp/internal/routes"
	"github.com/hayakawakaki/go-racp/internal/users/app"
	usersdomain "github.com/hayakawakaki/go-racp/internal/users/domain"
	"github.com/hayakawakaki/go-racp/server/config"
)

func middlewareCtxWithActor(ctx context.Context, userID int) context.Context {
	return middleware.ContextWithSnapshot(ctx, &middleware.AccountSnapshot{UserID: userID})
}

type stubService struct {
	listFn  func(context.Context, app.ListQuery) (app.UserPage, error)
	getFn   func(context.Context, int) (app.UserDetail, error)
	banFn   func(context.Context, app.BanCommand) (app.UserDetail, error)
	unbanFn func(context.Context, app.UnbanCommand) (app.UserDetail, error)
	roleFn  func(context.Context, app.SetRoleCommand) (app.UserDetail, error)
	allowed map[int]string
}

func (s *stubService) List(ctx context.Context, q app.ListQuery) (app.UserPage, error) {
	if s.listFn != nil {
		return s.listFn(ctx, q)
	}
	return app.UserPage{}, nil
}
func (s *stubService) Get(ctx context.Context, id int) (app.UserDetail, error) {
	if s.getFn != nil {
		return s.getFn(ctx, id)
	}
	return app.UserDetail{}, usersdomain.ErrNotFound
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

type stubSession struct{}

func (s *stubSession) Validate(_ context.Context, _ string) (*domain.Session, error) {
	return nil, domain.ErrSessionNotFound
}
func (s *stubSession) Destroy(_ context.Context, _ string) error { return nil }
func (s *stubSession) TTL() time.Duration                        { return time.Hour }

type stubUsers struct{}

func (s *stubUsers) GetByID(_ context.Context, id int) (*domain.User, error) {
	return &domain.User{ID: id}, nil
}

func newTestHandler() *Handler {
	return NewHandler(&stubService{}, HandlerConfig{
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		General: config.GeneralConfig{ServerName: "Test CP", Timezone: "UTC"},
	})
}

func TestHandler_RegisterRoutes_GatesListBehindAdmin(t *testing.T) {
	t.Parallel()
	h := newTestHandler()
	reg := routes.NewRegistry(
		config.AccessConfig{},
		domain.NewRoleResolver(config.RolesConfig{"Moderator": 20}),
		&stubSession{},
		&stubUsers{},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		false, true, httpx.Layout{},
	)
	mux := http.NewServeMux()
	h.RegisterRoutes(reg, mux)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/users", http.NoBody)
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("anonymous on /admin/users must 404; got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestHandler_ShowList_RendersUsernames(t *testing.T) {
	t.Parallel()
	svc := &stubService{
		listFn: func(_ context.Context, _ app.ListQuery) (app.UserPage, error) {
			return app.UserPage{
				Users: []usersdomain.User{
					{ID: 7, Username: "kaki", Email: "kaki@example.com"},
					{ID: 8, Username: "crazyarashi", Email: "crazy@example.com"},
				},
				Total: 2, Page: 1, PerPage: 20, TotalPages: 1,
			}, nil
		},
	}
	h := NewHandler(svc, HandlerConfig{
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		General: config.GeneralConfig{ServerName: "Test CP", Timezone: "UTC"},
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/users", http.NoBody)
	h.showList(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "kaki") || !strings.Contains(body, "crazyarashi") {
		t.Errorf("body missing usernames:\n%s", body)
	}
	if !strings.Contains(body, `id="admin-shell"`) {
		t.Errorf("non-HTMX call should include admin shell")
	}
}

func TestHandler_ShowList_HTMXFragment(t *testing.T) {
	t.Parallel()
	svc := &stubService{
		listFn: func(_ context.Context, _ app.ListQuery) (app.UserPage, error) {
			return app.UserPage{Users: []usersdomain.User{{ID: 7, Username: "kaki"}}, Total: 1, Page: 1, PerPage: 20, TotalPages: 1}, nil
		},
	}
	h := NewHandler(svc, HandlerConfig{
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		General: config.GeneralConfig{ServerName: "Test CP", Timezone: "UTC"},
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/users", http.NoBody)
	req.Header.Set("HX-Request", "true")
	h.showList(rr, req)

	body := rr.Body.String()
	if strings.Contains(body, `id="admin-shell"`) {
		t.Errorf("HTMX call must not include layout chrome")
	}
	if !strings.Contains(body, "kaki") {
		t.Errorf("fragment missing username")
	}
}

func TestHandler_ShowDetail_RendersUsernameAndChars(t *testing.T) {
	t.Parallel()
	svc := &stubService{
		getFn: func(_ context.Context, _ int) (app.UserDetail, error) {
			return app.UserDetail{
				User:       &usersdomain.User{ID: 7, Username: "kaki", Email: "k@example.com"},
				Characters: []usersdomain.Character{{ID: 1, Name: "Aurora", Class: 1, BaseLevel: 50}},
			}, nil
		},
		allowed: map[int]string{0: "Player", 20: "Moderator"},
	}
	h := NewHandler(svc, HandlerConfig{
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		General: config.GeneralConfig{ServerName: "Test CP", Timezone: "UTC"},
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/users/7", http.NoBody)
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
			return app.UserDetail{}, usersdomain.ErrNotFound
		},
	}
	h := NewHandler(svc, HandlerConfig{
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		General: config.GeneralConfig{ServerName: "Test CP", Timezone: "UTC"},
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/users/9999", http.NoBody)
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
			if cmd.PresetCode != "1h" || cmd.Reason != "spam" {
				t.Errorf("unexpected cmd: %+v", cmd)
			}
			return app.UserDetail{User: &usersdomain.User{ID: 7, Username: "kaki", State: 0}}, nil
		},
	}
	h := NewHandler(svc, HandlerConfig{
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		General: config.GeneralConfig{ServerName: "Test CP", Timezone: "UTC"},
	})

	rr := httptest.NewRecorder()
	body := strings.NewReader("preset=1h&reason=spam")
	req := httptest.NewRequest(http.MethodPost, "/admin/users/7/ban", body)
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
			return app.UserDetail{}, usersdomain.ErrEmptyReason
		},
	}
	h := NewHandler(svc, HandlerConfig{
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		General: config.GeneralConfig{ServerName: "Test CP", Timezone: "UTC"},
	})

	rr := httptest.NewRecorder()
	body := strings.NewReader("preset=1h")
	req := httptest.NewRequest(http.MethodPost, "/admin/users/7/ban", body)
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
			return app.UserDetail{User: &usersdomain.User{ID: 7, Username: "kaki"}}, nil
		},
	}
	h := NewHandler(svc, HandlerConfig{
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		General: config.GeneralConfig{ServerName: "Test CP", Timezone: "UTC"},
	})

	rr := httptest.NewRecorder()
	body := strings.NewReader("reason=appeal")
	req := httptest.NewRequest(http.MethodPost, "/admin/users/7/unban", body)
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
			return app.UserDetail{User: &usersdomain.User{ID: 7, Username: "kaki", GroupID: 20}}, nil
		},
		allowed: map[int]string{0: "Player", 20: "Moderator"},
	}
	h := NewHandler(svc, HandlerConfig{
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		General: config.GeneralConfig{ServerName: "Test CP", Timezone: "UTC"},
	})

	rr := httptest.NewRecorder()
	body := strings.NewReader("group_id=20&reason=promote")
	req := httptest.NewRequest(http.MethodPost, "/admin/users/7/role", body)
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

func TestTierBadge_LabelsForEachTier(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		unban    time.Time
		expected string
		state    int
	}{
		{time.Time{}, "Active", 0},
		{time.Time{}, "Unverified", 1},
		{time.Time{}, "Perma Ban", 5},
		{now.Add(time.Hour), "Temp Ban", 0},
	}
	for _, tc := range cases {
		t.Run(tc.expected, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			user := &usersdomain.User{State: tc.state, UnbanTime: tc.unban}
			if err := tierBadge(user, now).Render(req.Context(), w); err != nil {
				t.Fatalf("render: %v", err)
			}
			if !strings.Contains(w.Body.String(), tc.expected) {
				t.Errorf("body=%q want %q", w.Body.String(), tc.expected)
			}
		})
	}
}
