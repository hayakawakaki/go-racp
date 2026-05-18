package transport

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/httpx"
	mobapp "github.com/hayakawakaki/go-racp/internal/mob/app"
	"github.com/hayakawakaki/go-racp/internal/mob/domain"
	"github.com/hayakawakaki/go-racp/internal/routes"
	"github.com/hayakawakaki/go-racp/server/config"
)

type mobService interface {
	Get(ctx context.Context, id int) (*domain.Mob, error)
	List(ctx context.Context, query mobapp.ListQuery) (mobapp.MobPage, error)
	Reload(ctx context.Context) error
	Status() mobapp.ServiceStatus
}

type HandlerConfig struct {
	Logger  *slog.Logger
	General config.GeneralConfig
}

type Handler struct {
	svc     mobService
	logger  *slog.Logger
	general config.GeneralConfig
}

func NewHandler(svc mobService, cfg HandlerConfig) *Handler {
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
	reg.Public(mux, "GET /mobs", http.HandlerFunc(h.showList))
	reg.Public(mux, "GET /mobs/{id}", http.HandlerFunc(h.showDetail))
	reg.Public(mux, "GET /api/mobs/{id}", http.HandlerFunc(h.apiDetail))
	reg.Wrap(mux, "Admin.MobsReload", "POST /admin/mobs/reload", http.HandlerFunc(h.doReload))
}
