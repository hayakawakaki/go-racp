package transport

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	accdomain "github.com/hayakawakaki/go-racp/internal/features/account/domain"
	"github.com/hayakawakaki/go-racp/internal/features/account/transport/middleware"
	"github.com/hayakawakaki/go-racp/internal/features/news/app"
	"github.com/hayakawakaki/go-racp/internal/features/news/domain"
	"github.com/hayakawakaki/go-racp/internal/features/news/infra"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
	"github.com/hayakawakaki/go-racp/server/config"
)

type newsService interface {
	Categories() domain.CategoryResolver
	Create(ctx context.Context, title, body, category string) (int64, error)
	Update(ctx context.Context, id int64, title, body, category string) error
	Delete(ctx context.Context, id int64) error
	GetByID(ctx context.Context, id int64) (app.NewsItem, error)
	List(ctx context.Context) ([]app.NewsItem, error)
	ListByCategory(ctx context.Context, category string) ([]app.NewsItem, error)
}

type userLookup interface {
	GetByID(ctx context.Context, id int) (*accdomain.User, error)
}

type Renderer interface {
	NewsListPage(layout httpx.Layout, state NewsListState) templ.Component
	NewsDetailPage(layout httpx.Layout, state NewsDetailState) templ.Component
	NewsFormPage(layout httpx.Layout, state NewsFormState) templ.Component
}

//nolint:govet // GeneralConfig trailing bool forces alignment cost
type HandlerConfig struct {
	General     config.GeneralConfig
	Users       userLookup
	Roles       accdomain.RoleResolver
	Theme       Renderer
	Logger      *slog.Logger
	ManageRoles []string
}

//nolint:govet // GeneralConfig trailing bool forces alignment cost
type Handler struct {
	general     config.GeneralConfig
	svc         newsService
	renderer    *infra.Renderer
	users       userLookup
	roles       accdomain.RoleResolver
	manageRoles map[string]struct{}
	theme       Renderer
	logger      *slog.Logger
}

func NewHandler(service newsService, renderer *infra.Renderer, cfg HandlerConfig) *Handler {
	manageRoles := make(map[string]struct{}, len(cfg.ManageRoles))
	for _, name := range cfg.ManageRoles {
		manageRoles[name] = struct{}{}
	}

	return &Handler{
		svc:         service,
		renderer:    renderer,
		logger:      cfg.Logger,
		users:       cfg.Users,
		roles:       cfg.Roles,
		manageRoles: manageRoles,
		general:     cfg.General,
		theme:       cfg.Theme,
	}
}

func (h *Handler) layout() httpx.Layout {
	return httpx.Layout{GeneralConfig: h.general}
}

func (h *Handler) canManage(r *http.Request) bool {
	var groupID int
	if snap, ok := middleware.SnapshotFromContext(r.Context()); ok {
		groupID = snap.GroupID
	} else {
		session, ok := middleware.SessionFromContext(r.Context())
		if !ok {
			return false
		}
		if h.users == nil {
			return false
		}
		user, err := h.users.GetByID(r.Context(), session.UserID)
		if err != nil || user == nil {
			return false
		}
		groupID = user.GroupID
	}

	role := h.roles.Resolve(groupID)
	if role == accdomain.RoleAdmin {
		return true
	}
	_, ok := h.manageRoles[role.Name]

	return ok
}

func (h *Handler) RegisterRoutes(reg *routes.Registry, mux *http.ServeMux) {
	reg.Wrap(mux, "News.View", "GET /news", http.HandlerFunc(h.htmlList))
	reg.Wrap(mux, "News.Manage", "GET /news/create", http.HandlerFunc(h.htmlCreateForm))
	reg.Wrap(mux, "News.Manage", "GET /news/{id}/edit", http.HandlerFunc(h.htmlEditForm))
	reg.Wrap(mux, "News.View", "GET /news/{id}", http.HandlerFunc(h.htmlDetail))
	reg.Wrap(mux, "News.Manage", "POST /news", http.HandlerFunc(h.htmlCreate))
	reg.Wrap(mux, "News.Manage", "POST /news/preview", http.HandlerFunc(h.htmlPreview))
	reg.Wrap(mux, "News.Manage", "POST /news/{id}/edit", http.HandlerFunc(h.htmlUpdate))
	reg.Wrap(mux, "News.Manage", "POST /news/{id}/delete", http.HandlerFunc(h.htmlDelete))

	// Public API
	reg.Wrap(mux, "News.View", "GET /api/v1/news", http.HandlerFunc(h.jsonList))
	reg.Wrap(mux, "News.View", "GET /api/v1/news/{id}", http.HandlerFunc(h.jsonGet))
}
