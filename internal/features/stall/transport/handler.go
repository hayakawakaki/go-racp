package transport

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	itemdomain "github.com/hayakawakaki/go-racp/internal/features/item/domain"
	"github.com/hayakawakaki/go-racp/internal/features/stall/domain"
	"github.com/hayakawakaki/go-racp/internal/features/stall/transport/state"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
	"github.com/hayakawakaki/go-racp/server/config"
)

type vendorService interface {
	List(ctx context.Context, q domain.ListQuery) (domain.Page, error)
	Get(ctx context.Context, key domain.VendorKey) (domain.Vendor, error)
}

type itemLookup interface {
	Get(ctx context.Context, id int) (*itemdomain.Item, error)
	Loaded() bool
}

type Renderer interface {
	StallListPage(layout httpx.Layout, state state.ListState) templ.Component
	StallListContent(state state.ListState) templ.Component
	StallLoadingPage(layout httpx.Layout, refreshURL string) templ.Component
	StallLoadingContent(refreshURL string) templ.Component
	StallVendingBox(state state.StallState) templ.Component
}

//nolint:govet // GeneralConfig trailing bool forces alignment cost
type HandlerConfig struct {
	General    config.GeneralConfig
	ItemLookup itemLookup
	Theme      Renderer
	Logger     *slog.Logger
}

//nolint:govet // GeneralConfig trailing bool forces alignment cost
type Handler struct {
	general    config.GeneralConfig
	svc        vendorService
	itemLookup itemLookup
	theme      Renderer
	logger     *slog.Logger
}

func NewHandler(svc vendorService, cfg HandlerConfig) *Handler {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Handler{
		svc:        svc,
		logger:     logger,
		itemLookup: cfg.ItemLookup,
		general:    cfg.General,
		theme:      cfg.Theme,
	}
}

func (h *Handler) layout() httpx.Layout {
	return httpx.Layout{GeneralConfig: h.general}
}

func (h *Handler) RegisterRoutes(reg *routes.Registry, mux *http.ServeMux) {
	reg.Wrap(mux, "Vendor.View", "GET /vendors", http.HandlerFunc(h.showList))
	reg.Wrap(mux, "Vendor.View", "GET /vendors/{type}/{id}/items", http.HandlerFunc(h.showStallItems))
}
