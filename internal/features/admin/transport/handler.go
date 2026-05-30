package transport

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/a-h/templ"
	currency "github.com/hayakawakaki/go-racp/internal/features/account/app/currency"
	modapp "github.com/hayakawakaki/go-racp/internal/features/account/app/moderation"
	"github.com/hayakawakaki/go-racp/internal/features/account/transport/middleware"
	modstate "github.com/hayakawakaki/go-racp/internal/features/account/transport/moderation/state"
	"github.com/hayakawakaki/go-racp/internal/features/admin/transport/state"
	guildapp "github.com/hayakawakaki/go-racp/internal/features/guild/app"
	guildstate "github.com/hayakawakaki/go-racp/internal/features/guild/transport/state"
	itemapp "github.com/hayakawakaki/go-racp/internal/features/item/app"
	mobapp "github.com/hayakawakaki/go-racp/internal/features/mob/app"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
	"github.com/hayakawakaki/go-racp/internal/platform/metric/domain"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
	"github.com/hayakawakaki/go-racp/server/config"
	"golang.org/x/sync/errgroup"
)

const economyPerPage = 15

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

type userLister interface {
	List(ctx context.Context, query modapp.ListQuery) (modapp.UserPage, error)
}

type guildLister interface {
	List(ctx context.Context, query guildapp.ListQuery) (guildapp.GuildPage, error)
}

type economyReader interface {
	Totals(ctx context.Context) (currency.TotalsDTO, error)
	DepositHistory(ctx context.Context, page, perPage int) (currency.DepositPage, error)
	WithdrawHistory(ctx context.Context, page, perPage int) (currency.WithdrawHistoryPage, error)
	StuckWithdraws(ctx context.Context) ([]currency.AdminWithdrawDTO, error)
}

type emailResolver interface {
	EmailsByIDs(ctx context.Context, ids []int) (map[int]string, error)
}

type Renderer interface {
	AdminLayout(layout httpx.Layout, pageTitle string, content templ.Component) templ.Component
	DashboardContent(state state.DashboardState) templ.Component
	DatabaseContent(state state.DatabaseState) templ.Component
	UsersListPage(layout httpx.Layout, state modstate.ListState) templ.Component
	UsersListContent(state modstate.ListState) templ.Component
	GuildListPage(layout httpx.Layout, state guildstate.ListState) templ.Component
	GuildListContent(state guildstate.ListState) templ.Component
	EconomyContent(state state.EconomyState) templ.Component
}

//nolint:govet // GeneralConfig trailing bool forces alignment cost
type HandlerConfig struct {
	General    config.GeneralConfig
	ItemStatus itemStatusProvider
	MobStatus  mobStatusProvider
	Metric     metricReader
	Users      userLister
	Guilds     guildLister
	Economy    economyReader
	Emails     emailResolver
	Theme      Renderer
	Logger     *slog.Logger
}

//nolint:govet // GeneralConfig trailing bool forces alignment cost
type Handler struct {
	general    config.GeneralConfig
	itemStatus itemStatusProvider
	mobStatus  mobStatusProvider
	metric     metricReader
	users      userLister
	guilds     guildLister
	economy    economyReader
	emails     emailResolver
	theme      Renderer
	logger     *slog.Logger
}

func NewHandler(cfg HandlerConfig) *Handler {
	return &Handler{
		logger:     cfg.Logger,
		itemStatus: cfg.ItemStatus,
		mobStatus:  cfg.MobStatus,
		metric:     cfg.Metric,
		users:      cfg.Users,
		guilds:     cfg.Guilds,
		economy:    cfg.Economy,
		emails:     cfg.Emails,
		general:    cfg.General,
		theme:      cfg.Theme,
	}
}

func (h *Handler) layout() httpx.Layout {
	return httpx.Layout{GeneralConfig: h.general}
}

func (h *Handler) RegisterRoutes(reg *routes.Registry, mux *http.ServeMux) {
	reg.Wrap(mux, "Admin.Dashboard", "GET /admin", http.HandlerFunc(h.showDashboard))
	reg.Wrap(mux, "Admin.Users", "GET /admin/users", http.HandlerFunc(h.showUsers))
	reg.Wrap(mux, "Admin.Guilds", "GET /admin/guilds", http.HandlerFunc(h.showGuilds))
	reg.Wrap(mux, "Admin.Database", "GET /admin/database", http.HandlerFunc(h.showDatabase))
	reg.Wrap(mux, "Admin.Economy", "GET /admin/economy", http.HandlerFunc(h.showEconomy))
}

func (h *Handler) showUsers(w http.ResponseWriter, r *http.Request) {
	query := modapp.ListQuery{
		Page:    httpx.ParsePositiveInt(r.URL.Query().Get("page"), 1),
		PerPage: 15,
		Query:   r.URL.Query().Get("q"),
	}
	if snap, ok := middleware.SnapshotFromContext(r.Context()); ok && snap != nil {
		query.ExcludeID = snap.UserID
	}

	page, err := h.users.List(r.Context(), query)
	if err != nil {
		h.logger.Error("admin: users list failed", "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	s := modstate.ListState{Page: page, Query: query.Query, BaseURL: "/admin/users", Now: time.Now()}
	if httpx.IsHTMX(r) {
		httpx.RenderHTML(w, r, h.logger, h.theme.UsersListContent(s))
		return
	}
	httpx.RenderHTML(w, r, h.logger, h.theme.UsersListPage(h.layout(), s))
}

func (h *Handler) showGuilds(w http.ResponseWriter, r *http.Request) {
	query := guildapp.ListQuery{
		Page:    httpx.ParsePositiveInt(r.URL.Query().Get("page"), 1),
		PerPage: 15,
		Query:   r.URL.Query().Get("q"),
	}

	page, err := h.guilds.List(r.Context(), query)
	if err != nil {
		h.logger.Error("admin: guilds list failed", "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	s := guildstate.ListState{Page: page, Query: query.Query, BaseURL: "/admin/guilds"}
	if httpx.IsHTMX(r) {
		httpx.RenderHTML(w, r, h.logger, h.theme.GuildListContent(s))
		return
	}
	httpx.RenderHTML(w, r, h.logger, h.theme.GuildListPage(h.layout(), s))
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
			s.PeaksFailed = true
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

func (h *Handler) showEconomy(w http.ResponseWriter, r *http.Request) {
	dpage := httpx.ParsePositiveInt(r.URL.Query().Get("dpage"), 1)
	wpage := httpx.ParsePositiveInt(r.URL.Query().Get("wpage"), 1)
	s := h.economyState(r.Context(), dpage, wpage)
	if httpx.IsHTMX(r) {
		httpx.RenderHTML(w, r, h.logger, h.theme.EconomyContent(s))
		return
	}
	httpx.RenderHTML(w, r, h.logger, h.theme.AdminLayout(h.layout(), "Economy", h.theme.EconomyContent(s)))
}

func (h *Handler) economyState(ctx context.Context, dpage, wpage int) state.EconomyState {
	s := state.EconomyState{Location: h.general.Location()}
	if h.economy == nil {
		return s
	}

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		totals, err := h.economy.Totals(gctx)
		if err != nil {
			h.logger.Warn("admin: economy totals read failed", "err", err)
			s.TotalsFailed = true
			return nil
		}
		s.Totals = totals
		return nil
	})
	g.Go(func() error {
		deposits, err := h.economy.DepositHistory(gctx, dpage, economyPerPage)
		if err != nil {
			h.logger.Warn("admin: economy deposits read failed", "err", err)
			s.DepositsFailed = true
			return nil
		}
		s.Deposits = deposits
		return nil
	})
	g.Go(func() error {
		withdraws, err := h.economy.WithdrawHistory(gctx, wpage, economyPerPage)
		if err != nil {
			h.logger.Warn("admin: economy withdraws read failed", "err", err)
			s.WithdrawsFailed = true
			return nil
		}
		s.Withdraws = withdraws
		return nil
	})
	g.Go(func() error {
		stuck, err := h.economy.StuckWithdraws(gctx)
		if err != nil {
			h.logger.Warn("admin: economy stuck withdraws read failed", "err", err)
			return nil
		}
		s.Stuck = stuck
		return nil
	})
	_ = g.Wait()

	h.resolveEconomyEmails(ctx, s.Deposits.Rows, s.Withdraws.Rows)

	return s
}

func (h *Handler) resolveEconomyEmails(ctx context.Context, deposits []currency.DepositDTO, withdraws []currency.AdminWithdrawDTO) {
	if h.emails == nil {
		return
	}

	ids := make([]int, 0, len(deposits)+len(withdraws))
	for _, row := range deposits {
		ids = append(ids, row.AccountID)
	}
	for _, row := range withdraws {
		ids = append(ids, row.AccountID)
	}
	if len(ids) == 0 {
		return
	}

	emails, err := h.emails.EmailsByIDs(ctx, ids)
	if err != nil {
		h.logger.Warn("admin: economy email lookup failed", "err", err)
		return
	}

	for index := range deposits {
		deposits[index].Email = emails[deposits[index].AccountID]
	}
	for index := range withdraws {
		withdraws[index].Email = emails[withdraws[index].AccountID]
	}
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
