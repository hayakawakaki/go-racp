package transport

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	"github.com/hayakawakaki/go-racp/internal/features/billing/domain"
	"github.com/hayakawakaki/go-racp/internal/features/billing/transport/state"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
	"github.com/hayakawakaki/go-racp/server/config"
)

const (
	maxCheckoutFormBytes = 1 << 10

	fieldPackage = "package"
)

type billingService interface {
	Packages() []domain.Package
	Available() bool
	StartCheckout(ctx context.Context, accountID int, packageKey, successURL, cancelURL string) (string, error)
}

type Renderer interface {
	StorePage(layout httpx.Layout, state state.StoreState) templ.Component
}

//nolint:govet // GeneralConfig trailing bool forces alignment cost
type HandlerConfig struct {
	Logger   *slog.Logger
	Theme    Renderer
	Currency string
	AppURL   string
	General  config.GeneralConfig
}

//nolint:govet // GeneralConfig trailing bool forces alignment cost
type Handler struct {
	svc      billingService
	theme    Renderer
	logger   *slog.Logger
	currency string
	appURL   string
	general  config.GeneralConfig
}

func NewHandler(svc billingService, cfg HandlerConfig) *Handler {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Handler{
		svc:      svc,
		theme:    cfg.Theme,
		logger:   logger,
		currency: cfg.Currency,
		appURL:   cfg.AppURL,
		general:  cfg.General,
	}
}

func (h *Handler) layout() httpx.Layout {
	return httpx.Layout{GeneralConfig: h.general}
}

func (h *Handler) RegisterRoutes(reg *routes.Registry, mux *http.ServeMux) {
	reg.Wrap(mux, "Store.View", "GET /store", http.HandlerFunc(h.showStore))
	reg.Wrap(mux, "Store.Checkout", "POST /store/checkout", http.HandlerFunc(h.startCheckout))
}
