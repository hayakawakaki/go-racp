package transport

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	itemdomain "github.com/hayakawakaki/go-racp/internal/features/item/domain"
	"github.com/hayakawakaki/go-racp/internal/features/mob/app"
	"github.com/hayakawakaki/go-racp/internal/features/mob/domain"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
	"github.com/hayakawakaki/go-racp/server/config"
)

type mobService interface {
	Get(ctx context.Context, id int) (*domain.Mob, error)
	List(ctx context.Context, query app.ListQuery) (app.MobPage, error)
	Reload(ctx context.Context) error
	Status() app.ServiceStatus
}

type ItemLookup interface {
	LookupByAegis(aegis string) *itemdomain.Item
}

type Renderer interface {
	MobDetailPage(layout httpx.Layout, state DetailState) templ.Component
	MobListPage(layout httpx.Layout, state ListState) templ.Component
	MobNotFoundPage(layout httpx.Layout, id string) templ.Component
	MobEmptyDatabasePage(layout httpx.Layout) templ.Component
	MobReloadSuccess(status app.ServiceStatus) templ.Component
	MobReloadFailure(message string) templ.Component
	MobReloadConflict() templ.Component
}

//nolint:govet // GeneralConfig trailing bool forces alignment cost
type HandlerConfig struct {
	General      config.GeneralConfig
	ItemLookupFn func() ItemLookup
	Theme        Renderer
	Logger       *slog.Logger
}

//nolint:govet // GeneralConfig trailing bool forces alignment cost
type Handler struct {
	general      config.GeneralConfig
	svc          mobService
	itemLookupFn func() ItemLookup
	theme        Renderer
	logger       *slog.Logger
}

func NewHandler(svc mobService, cfg HandlerConfig) *Handler {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Handler{
		svc:          svc,
		logger:       logger,
		itemLookupFn: cfg.ItemLookupFn,
		general:      cfg.General,
		theme:        cfg.Theme,
	}
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
