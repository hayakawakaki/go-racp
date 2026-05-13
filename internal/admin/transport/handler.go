package transport

import (
	"log/slog"
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/httpx"
	"github.com/hayakawakaki/go-racp/server/config"
)

type HandlerConfig struct {
	Logger  *slog.Logger
	General config.GeneralConfig
}

type Handler struct {
	logger  *slog.Logger
	general config.GeneralConfig
}

func NewHandler(cfg HandlerConfig) *Handler {
	return &Handler{
		logger:  cfg.Logger,
		general: cfg.General,
	}
}

func (h *Handler) layout() httpx.Layout {
	return httpx.Layout{GeneralConfig: h.general}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAdmin func(http.Handler) http.Handler) {
	mux.Handle("GET /admin", requireAdmin(http.HandlerFunc(h.showDashboard)))
}

func (h *Handler) showDashboard(w http.ResponseWriter, r *http.Request) {
	if httpx.IsHTMX(r) {
		httpx.RenderHTML(w, r, h.logger, dashboardContent())
		return
	}
	httpx.RenderHTML(w, r, h.logger, adminLayout(h.layout(), "Dashboard", dashboardContent()))
}
