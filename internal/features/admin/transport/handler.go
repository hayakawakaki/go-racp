package transport

import (
	"context"
	"log/slog"
	"net/http"

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

type HandlerConfig struct {
	Logger     *slog.Logger
	ItemStatus itemStatusProvider
	MobStatus  mobStatusProvider
	Metric     metricReader
	General    config.GeneralConfig
}

type Handler struct {
	logger     *slog.Logger
	itemStatus itemStatusProvider
	mobStatus  mobStatusProvider
	metric     metricReader
	general    config.GeneralConfig
}

func NewHandler(cfg HandlerConfig) *Handler {
	return &Handler{
		logger:     cfg.Logger,
		itemStatus: cfg.ItemStatus,
		mobStatus:  cfg.MobStatus,
		metric:     cfg.Metric,
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
	state := h.dashboardState(r.Context())
	if httpx.IsHTMX(r) {
		httpx.RenderHTML(w, r, h.logger, dashboardContent(state))
		return
	}
	httpx.RenderHTML(w, r, h.logger, AdminLayout(h.layout(), "Dashboard", dashboardContent(state)))
}

func (h *Handler) dashboardState(ctx context.Context) dashboardState {
	state := dashboardState{}
	if h.metric == nil {
		return state
	}
	state.Online = h.metric.Online(ctx)

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

	state.General = general
	state.PeakTable = buildPeakTable(peaks)
	return state
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
