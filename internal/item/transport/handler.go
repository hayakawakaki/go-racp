package transport

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/httpx"
	itemapp "github.com/hayakawakaki/go-racp/internal/item/app"
	"github.com/hayakawakaki/go-racp/internal/item/domain"
	"github.com/hayakawakaki/go-racp/internal/routes"
	"github.com/hayakawakaki/go-racp/server/config"
)

type itemService interface {
	Get(ctx context.Context, id int) (*domain.Item, error)
	List(ctx context.Context, query itemapp.ListQuery) (itemapp.ItemPage, error)
	Reload(ctx context.Context) error
	Status() itemapp.ServiceStatus
}

type HandlerConfig struct {
	Logger  *slog.Logger
	General config.GeneralConfig
}

type Handler struct {
	svc     itemService
	logger  *slog.Logger
	general config.GeneralConfig
}

func NewHandler(svc itemService, cfg HandlerConfig) *Handler {
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
	reg.Public(mux, "GET /items", http.HandlerFunc(h.showList))
	reg.Public(mux, "GET /items/{id}", http.HandlerFunc(h.showDetail))
	reg.Public(mux, "GET /api/items/{id}", http.HandlerFunc(h.apiDetail))
	reg.Wrap(mux, "Admin.ItemsReload", "POST /admin/items/reload", http.HandlerFunc(h.doReload))
}
