package transport

import (
	"context"
	"log/slog"
	"net/http"

	itemdomain "github.com/hayakawakaki/go-racp/internal/features/item/domain"
	"github.com/hayakawakaki/go-racp/internal/features/stall/domain"
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
}

type HandlerConfig struct {
	Logger     *slog.Logger
	ItemLookup itemLookup
	General    config.GeneralConfig
}

type Handler struct {
	svc        vendorService
	logger     *slog.Logger
	itemLookup itemLookup
	general    config.GeneralConfig
}

func NewHandler(svc vendorService, cfg HandlerConfig) *Handler {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Handler{svc: svc, logger: logger, itemLookup: cfg.ItemLookup, general: cfg.General}
}

func (h *Handler) layout() httpx.Layout {
	return httpx.Layout{GeneralConfig: h.general}
}

func (h *Handler) RegisterRoutes(reg *routes.Registry, mux *http.ServeMux) {
	reg.Wrap(mux, "Vendor.View", "GET /vendors", http.HandlerFunc(h.showList))
	reg.Wrap(mux, "Vendor.View", "GET /vendors/{type}/{id}/items", http.HandlerFunc(h.showStallItems))
}
