package transport

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	"github.com/hayakawakaki/go-racp/internal/features/character/app"
	"github.com/hayakawakaki/go-racp/internal/features/character/transport/state"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
	"github.com/hayakawakaki/go-racp/server/config"
)

type characterService interface {
	Get(ctx context.Context, accountID, charID int) (*app.CharacterDTO, error)
	ResetLook(ctx context.Context, accountID, charID int) error
	ResetLocation(ctx context.Context, accountID, charID int) error
}

type Renderer interface {
	CharacterDetailPage(layout httpx.Layout, state state.DetailState) templ.Component
}

//nolint:govet // GeneralConfig trailing bool forces alignment cost
type HandlerConfig struct {
	General config.GeneralConfig
	Theme   Renderer
	Logger  *slog.Logger
}

//nolint:govet // GeneralConfig trailing bool forces alignment cost
type Handler struct {
	general config.GeneralConfig
	svc     characterService
	theme   Renderer
	logger  *slog.Logger
}

func NewHandler(svc characterService, cfg HandlerConfig) *Handler {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Handler{
		svc:     svc,
		logger:  logger,
		general: cfg.General,
		theme:   cfg.Theme,
	}
}

func (h *Handler) layout() httpx.Layout {
	return httpx.Layout{GeneralConfig: h.general}
}

func (h *Handler) RegisterRoutes(reg *routes.Registry, mux *http.ServeMux) {
	reg.Wrap(mux, "Character.View", "GET /characters/{charID}", http.HandlerFunc(h.showDetail))
	reg.Wrap(mux, "Character.Reset", "POST /characters/{charID}/look", http.HandlerFunc(h.doResetLook))
	reg.Wrap(mux, "Character.Reset", "POST /characters/{charID}/location", http.HandlerFunc(h.doResetLocation))
}
