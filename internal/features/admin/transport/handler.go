package transport

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	"github.com/hayakawakaki/go-racp/internal/features/admin/transport/state"
	itemapp "github.com/hayakawakaki/go-racp/internal/features/item/app"
	mobapp "github.com/hayakawakaki/go-racp/internal/features/mob/app"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
	"github.com/hayakawakaki/go-racp/internal/platform/metric/domain"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
	"github.com/hayakawakaki/go-racp/server/config"
	"golang.org/x/sync/errgroup"
)

type itemStatusProvider interface {
	Status() itemapp.ServiceStatus
}

type mobStatusProvider interface {
	Status() mobapp.ServiceStatus
}

type metricReader interface {
	Online(ctx context.Context) domain.OnlineSnapshot
	Peaks(ctx context.Context) ([]domain.PeakRow, error)
	General(ctx context.Context) (domain.GeneralSnapshot, error)
}

type Renderer interface {
	AdminLayout(layout httpx.Layout, pageTitle string, content templ.Component) templ.Component
	DashboardContent(state state.DashboardState) templ.Component
	DatabaseContent(state state.DatabaseState) templ.Component
}

//nolint:govet // GeneralConfig trailing bool forces alignment cost
type HandlerConfig struct {
	General    config.GeneralConfig
	ItemStatus itemStatusProvider
	MobStatus  mobStatusProvider
	Metric     metricReader
	Theme      Renderer
	Logger     *slog.Logger
}

//nolint:govet // GeneralConfig trailing bool forces alignment cost
type Handler struct {
	general    config.GeneralConfig
	itemStatus itemStatusProvider
	mobStatus  mobStatusProvider
	metric     metricReader
	theme      Renderer
	logger     *slog.Logger
}

func NewHandler(cfg HandlerConfig) *Handler {
	return &Handler{
		logger:     cfg.Logger,
		itemStatus: cfg.ItemStatus,
		mobStatus:  cfg.MobStatus,
		metric:     cfg.Metric,
		general:    cfg.General,
		theme:      cfg.Theme,
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
	s := h.dashboardState(r.Context())
	if httpx.IsHTMX(r) {
		httpx.RenderHTML(w, r, h.logger, h.theme.DashboardContent(s))
		return
	}
	httpx.RenderHTML(w, r, h.logger, h.theme.AdminLayout(h.layout(), "Dashboard", h.theme.DashboardContent(s)))
}

func (h *Handler) dashboardState(ctx context.Context) state.DashboardState {
	s := state.DashboardState{}
	if h.metric == nil {
		return s
	}
	s.Online = h.metric.Online(ctx)

	var general domain.GeneralSnapshot
	var peaks []domain.PeakRow
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		snap, err := h.metric.General(gctx)
		if err != nil {
			h.logger.Warn("admin: general snapshot read failed", "err", err)
			return nil
		}
		general = snap
		return nil
	})
	g.Go(func() error {
		rows, err := h.metric.Peaks(gctx)
		if err != nil {
			h.logger.Warn("admin: peaks read failed", "err", err)
			return nil
		}
		peaks = rows
		return nil
	})
	_ = g.Wait()

	s.General = general
	s.PeakTable = state.BuildPeakTable(peaks)
	return s
}

func (h *Handler) showDatabase(w http.ResponseWriter, r *http.Request) {
	s := h.databaseState()
	if httpx.IsHTMX(r) {
		httpx.RenderHTML(w, r, h.logger, h.theme.DatabaseContent(s))
		return
	}
	httpx.RenderHTML(w, r, h.logger, h.theme.AdminLayout(h.layout(), "Database", h.theme.DatabaseContent(s)))
}

func (h *Handler) databaseState() state.DatabaseState {
	s := state.DatabaseState{}
	if h.itemStatus != nil {
		s.Item = h.itemStatus.Status()
	}
	if h.mobStatus != nil {
		s.Mob = h.mobStatus.Status()
	}

	return s
}
