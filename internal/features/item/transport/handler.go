package transport

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	"github.com/hayakawakaki/go-racp/internal/features/item/app"
	"github.com/hayakawakaki/go-racp/internal/features/item/domain"
	"github.com/hayakawakaki/go-racp/internal/features/item/transport/state"
	mobdomain "github.com/hayakawakaki/go-racp/internal/features/mob/domain"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
	"github.com/hayakawakaki/go-racp/server/config"
)

type itemService interface {
	Get(ctx context.Context, id int) (*domain.Item, error)
	List(ctx context.Context, query app.ListQuery) (app.ItemPage, error)
	Reload(ctx context.Context) error
	Status() app.ServiceStatus
}

type dropLookup interface {
	WhoDrops(itemAegis string) []mobdomain.DropOf
}

type Renderer interface {
	ItemDetailPage(layout httpx.Layout, state state.DetailState) templ.Component
	ItemListPage(layout httpx.Layout, state state.ListState) templ.Component
	ItemNotFoundPage(layout httpx.Layout, id string) templ.Component
	ItemEmptyDatabasePage(layout httpx.Layout) templ.Component
	ItemReloadSuccess(status app.ServiceStatus) templ.Component
	ItemReloadFailure(message string) templ.Component
	ItemReloadConflict() templ.Component
}

//nolint:govet // GeneralConfig trailing bool forces alignment cost
type HandlerConfig struct {
	General    config.GeneralConfig
	DropLookup dropLookup
	Theme      Renderer
	Logger     *slog.Logger
}

//nolint:govet // GeneralConfig trailing bool forces alignment cost
type Handler struct {
	general    config.GeneralConfig
	svc        itemService
	dropLookup dropLookup
	theme      Renderer
	logger     *slog.Logger
}

func NewHandler(svc itemService, cfg HandlerConfig) *Handler {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Handler{
		svc:        svc,
		logger:     logger,
		dropLookup: cfg.DropLookup,
		general:    cfg.General,
		theme:      cfg.Theme,
	}
}

func (h *Handler) layout() httpx.Layout {
	return httpx.Layout{GeneralConfig: h.general}
}

func (h *Handler) RegisterRoutes(reg *routes.Registry, mux *http.ServeMux) {
	reg.Wrap(mux, "Item.View", "GET /items", http.HandlerFunc(h.showList))
	reg.Wrap(mux, "Item.View", "GET /items/{id}", http.HandlerFunc(h.showDetail))
	reg.Wrap(mux, "Item.API", "GET /api/items/{id}", http.HandlerFunc(h.apiDetail))
	reg.Wrap(mux, "Admin.ItemsReload", "POST /admin/items/reload", http.HandlerFunc(h.doReload))
}
