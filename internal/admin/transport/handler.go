package transport

import (
	"log/slog"
	"net/http"

	itemapp "github.com/hayakawakaki/go-racp/internal/features/item/app"
	"github.com/hayakawakaki/go-racp/internal/httpx"
	mobapp "github.com/hayakawakaki/go-racp/internal/mob/app"
	"github.com/hayakawakaki/go-racp/internal/routes"
	"github.com/hayakawakaki/go-racp/server/config"
)

type itemStatusProvider interface {
	Status() itemapp.ServiceStatus
}

type mobStatusProvider interface {
	Status() mobapp.ServiceStatus
}

type HandlerConfig struct {
	Logger     *slog.Logger
	ItemStatus itemStatusProvider
	MobStatus  mobStatusProvider
	General    config.GeneralConfig
}

type Handler struct {
	logger     *slog.Logger
	itemStatus itemStatusProvider
	mobStatus  mobStatusProvider
	general    config.GeneralConfig
}

func NewHandler(cfg HandlerConfig) *Handler {
	return &Handler{
		logger:     cfg.Logger,
		itemStatus: cfg.ItemStatus,
		mobStatus:  cfg.MobStatus,
		general:    cfg.General,
	}
}

func (h *Handler) layout() httpx.Layout {
	return httpx.Layout{GeneralConfig: h.general}
}

func (h *Handler) RegisterRoutes(reg *routes.Registry, mux *http.ServeMux) {
	reg.Wrap(mux, "Admin.Dashboard", "GET /admin", http.HandlerFunc(h.showDashboard))
	reg.Wrap(mux, "Admin.Database", "GET /admin/database", http.HandlerFunc(h.showDatabase))
}

func (h *Handler) showDashboard(w http.ResponseWriter, r *http.Request) {
	if httpx.IsHTMX(r) {
		httpx.RenderHTML(w, r, h.logger, dashboardContent())
		return
	}
	httpx.RenderHTML(w, r, h.logger, AdminLayout(h.layout(), "Dashboard", dashboardContent()))
}

func (h *Handler) showDatabase(w http.ResponseWriter, r *http.Request) {
	state := h.databaseState()
	if httpx.IsHTMX(r) {
		httpx.RenderHTML(w, r, h.logger, databaseContent(state))
		return
	}
	httpx.RenderHTML(w, r, h.logger, AdminLayout(h.layout(), "Database", databaseContent(state)))
}

func (h *Handler) databaseState() databaseState {
	state := databaseState{}
	if h.itemStatus != nil {
		state.Item = h.itemStatus.Status()
	}
	if h.mobStatus != nil {
		state.Mob = h.mobStatus.Status()
	}

	return state
}
