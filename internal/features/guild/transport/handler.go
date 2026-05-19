package transport

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/features/guild/app"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
	"github.com/hayakawakaki/go-racp/server/config"
)

type guildService interface {
	List(ctx context.Context, q app.ListQuery) (app.GuildPage, error)
	Get(ctx context.Context, id int) (app.GuildDetail, error)
	GetEmblem(ctx context.Context, id int) ([]byte, string, error)
}

type HandlerConfig struct {
	Logger  *slog.Logger
	General config.GeneralConfig
}

type Handler struct {
	svc     guildService
	logger  *slog.Logger
	general config.GeneralConfig
}

func NewHandler(svc guildService, cfg HandlerConfig) *Handler {
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
	reg.Wrap(mux, "Guild.View", "GET /guilds", http.HandlerFunc(h.showList))
	reg.Wrap(mux, "Guild.View", "GET /guilds/{id}", http.HandlerFunc(h.showDetail))
	reg.Wrap(mux, "Guild.View", "GET /guilds/{id}/emblem", http.HandlerFunc(h.showEmblem))
}
