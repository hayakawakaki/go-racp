package moderation

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	app "github.com/hayakawakaki/go-racp/internal/features/account/app/moderation"
	"github.com/hayakawakaki/go-racp/internal/features/account/transport/moderation/state"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
	"github.com/hayakawakaki/go-racp/server/config"
)

type userService interface {
	Get(ctx context.Context, id int) (app.UserDetail, error)
	Ban(ctx context.Context, cmd app.BanCommand) (app.UserDetail, error)
	Unban(ctx context.Context, cmd app.UnbanCommand) (app.UserDetail, error)
	SetRole(ctx context.Context, cmd app.SetRoleCommand) (app.UserDetail, error)
	AllowedRoles() map[int]string
}

type Renderer interface {
	UsersDetailPage(layout httpx.Layout, username string, state state.DetailState) templ.Component
	UsersDetailContent(state state.DetailState) templ.Component
	UsersNotFoundPage(layout httpx.Layout, id string) templ.Component
	UsersActionError(message string) templ.Component
}

//nolint:govet // GeneralConfig trailing bool forces alignment cost
type HandlerConfig struct {
	General config.GeneralConfig
	Theme   Renderer
	Logger  *slog.Logger
}

//nolint:govet // GeneralConfig trailing bool forces alignment cost
type Handler struct {
	general config.GeneralConfig
	svc     userService
	theme   Renderer
	logger  *slog.Logger
}

func NewHandler(svc userService, cfg HandlerConfig) *Handler {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Handler{svc: svc, logger: logger, general: cfg.General, theme: cfg.Theme}
}

func (h *Handler) layout() httpx.Layout {
	return httpx.Layout{GeneralConfig: h.general}
}

func (h *Handler) RegisterRoutes(reg *routes.Registry, mux *http.ServeMux) {
	reg.Wrap(mux, "Users.View", "GET /users/{id}", http.HandlerFunc(h.showDetail))
	reg.Wrap(mux, "Users.Ban", "POST /users/{id}/ban", http.HandlerFunc(h.doBan))
	reg.Wrap(mux, "Users.Unban", "POST /users/{id}/unban", http.HandlerFunc(h.doUnban))
	reg.Wrap(mux, "Users.SetRole", "POST /users/{id}/role", http.HandlerFunc(h.doSetRole))
}
