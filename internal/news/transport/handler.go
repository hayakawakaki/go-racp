package transport

import (
	"context"
	"log/slog"
	"net/http"

	accountdomain "github.com/hayakawakaki/go-racp/internal/account/domain"
	"github.com/hayakawakaki/go-racp/internal/account/transport/middleware"
	"github.com/hayakawakaki/go-racp/internal/httpx"
	newsapp "github.com/hayakawakaki/go-racp/internal/news/app"
	"github.com/hayakawakaki/go-racp/internal/news/domain"
	newsinfra "github.com/hayakawakaki/go-racp/internal/news/infra"
	"github.com/hayakawakaki/go-racp/internal/routes"
	"github.com/hayakawakaki/go-racp/server/config"
)

type newsService interface {
	Categories() domain.CategoryResolver
	Create(ctx context.Context, title, body, category string) (int64, error)
	Update(ctx context.Context, id int64, title, body, category string) error
	Delete(ctx context.Context, id int64) error
	GetByID(ctx context.Context, id int64) (newsapp.NewsItem, error)
	List(ctx context.Context) ([]newsapp.NewsItem, error)
	ListByCategory(ctx context.Context, category string) ([]newsapp.NewsItem, error)
}

type userLookup interface {
	GetByID(ctx context.Context, id int) (*accountdomain.User, error)
}

type HandlerConfig struct {
	Logger      *slog.Logger
	Users       userLookup
	Roles       accountdomain.RoleResolver
	General     config.GeneralConfig
	ManageRoles []string
}

type Handler struct {
	svc         newsService
	logger      *slog.Logger
	renderer    *newsinfra.Renderer
	users       userLookup
	roles       accountdomain.RoleResolver
	manageRoles map[string]struct{}
	general     config.GeneralConfig
}

func NewHandler(service newsService, renderer *newsinfra.Renderer, cfg HandlerConfig) *Handler {
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
	if role == accountdomain.RoleAdmin {
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

	reg.Wrap(mux, "News.View", "GET /api/v1/news", http.HandlerFunc(h.jsonList))
	reg.Wrap(mux, "News.View", "GET /api/v1/news/{id}", http.HandlerFunc(h.jsonGet))
}
