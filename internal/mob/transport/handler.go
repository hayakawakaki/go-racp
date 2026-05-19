package transport

import (
	"context"
	"log/slog"
	"net/http"

	itemdomain "github.com/hayakawakaki/go-racp/internal/features/item/domain"
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

type ItemLookup interface {
	LookupByAegis(aegis string) *itemdomain.Item
}

type HandlerConfig struct {
	Logger       *slog.Logger
	ItemLookupFn func() ItemLookup
	General      config.GeneralConfig
}

type Handler struct {
	svc          mobService
	logger       *slog.Logger
	itemLookupFn func() ItemLookup
	general      config.GeneralConfig
}

func NewHandler(svc mobService, cfg HandlerConfig) *Handler {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Handler{svc: svc, logger: logger, itemLookupFn: cfg.ItemLookupFn, general: cfg.General}
}

func (h *Handler) currentItemLookup() ItemLookup {
	if h.itemLookupFn == nil {
		return nil
	}

	return h.itemLookupFn()
}

func (h *Handler) layout() httpx.Layout {
	return httpx.Layout{GeneralConfig: h.general}
}

func (h *Handler) RegisterRoutes(reg *routes.Registry, mux *http.ServeMux) {
	reg.Wrap(mux, "Mob.View", "GET /mobs", http.HandlerFunc(h.showList))
	reg.Wrap(mux, "Mob.View", "GET /mobs/{id}", http.HandlerFunc(h.showDetail))
	reg.Wrap(mux, "Mob.API", "GET /api/mobs/{id}", http.HandlerFunc(h.apiDetail))
	reg.Wrap(mux, "Admin.MobsReload", "POST /admin/mobs/reload", http.HandlerFunc(h.doReload))
}
