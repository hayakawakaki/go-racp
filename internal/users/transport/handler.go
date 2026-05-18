package transport

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/httpx"
	"github.com/hayakawakaki/go-racp/internal/routes"
	"github.com/hayakawakaki/go-racp/internal/users/app"
	"github.com/hayakawakaki/go-racp/server/config"
)

type userService interface {
	List(ctx context.Context, q app.ListQuery) (app.UserPage, error)
	Get(ctx context.Context, id int) (app.UserDetail, error)
	Ban(ctx context.Context, cmd app.BanCommand) (app.UserDetail, error)
	Unban(ctx context.Context, cmd app.UnbanCommand) (app.UserDetail, error)
	SetRole(ctx context.Context, cmd app.SetRoleCommand) (app.UserDetail, error)
	AllowedRoles() map[int]string
}

type HandlerConfig struct {
	Logger  *slog.Logger
	General config.GeneralConfig
}

type Handler struct {
	svc     userService
	logger  *slog.Logger
	general config.GeneralConfig
}

func NewHandler(svc userService, cfg HandlerConfig) *Handler {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Handler{svc: svc, logger: logger, general: cfg.General}
}

func (h *Handler) layout() httpx.Layout {
	return httpx.Layout{GeneralConfig: h.general}
}

func (h *Handler) RegisterRoutes(reg *routes.Registry, mux *http.ServeMux) {
	reg.Wrap(mux, "Admin.UsersList", "GET /admin/users", http.HandlerFunc(h.showList))
	reg.Wrap(mux, "Admin.UserView", "GET /admin/users/{id}", http.HandlerFunc(h.showDetail))
	reg.Wrap(mux, "Admin.UserBan", "POST /admin/users/{id}/ban", http.HandlerFunc(h.doBan))
	reg.Wrap(mux, "Admin.UserUnban", "POST /admin/users/{id}/unban", http.HandlerFunc(h.doUnban))
	reg.Wrap(mux, "Admin.UserSetRole", "POST /admin/users/{id}/role", http.HandlerFunc(h.doSetRole))
}
