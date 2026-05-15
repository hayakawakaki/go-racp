package transport

import (
	"context"
	"log/slog"
	"net/http"

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

type HandlerConfig struct {
	Logger  *slog.Logger
	General config.GeneralConfig
}

type Handler struct {
	svc      newsService
	logger   *slog.Logger
	renderer *newsinfra.Renderer
	general  config.GeneralConfig
}

func NewHandler(service newsService, renderer *newsinfra.Renderer, cfg HandlerConfig) *Handler {
	return &Handler{
		svc:      service,
		renderer: renderer,
		logger:   cfg.Logger,
		general:  cfg.General,
	}
}

func (h *Handler) layout() httpx.Layout {
	return httpx.Layout{GeneralConfig: h.general}
}

func (h *Handler) RegisterRoutes(reg *routes.Registry, mux *http.ServeMux) {
	reg.Public(mux, "GET /news", http.HandlerFunc(h.htmlList))
	reg.Public(mux, "GET /news/{id}", http.HandlerFunc(h.htmlDetail))
	reg.Wrap(mux, "News.Manage", "POST /news", http.HandlerFunc(h.htmlCreate))
	reg.Wrap(mux, "News.Manage", "POST /news/{id}/edit", http.HandlerFunc(h.htmlUpdate))
	reg.Wrap(mux, "News.Manage", "POST /news/{id}/delete", http.HandlerFunc(h.htmlDelete))

	reg.Public(mux, "GET /api/v1/news", http.HandlerFunc(h.jsonList))
	reg.Public(mux, "GET /api/v1/news/{id}", http.HandlerFunc(h.jsonGet))
}
