package transport

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/features/character/app"
	"github.com/hayakawakaki/go-racp/internal/httpx"
	"github.com/hayakawakaki/go-racp/internal/routes"
	"github.com/hayakawakaki/go-racp/server/config"
)

type characterService interface {
	Get(ctx context.Context, accountID, charID int) (*app.CharacterDTO, error)
	ResetLook(ctx context.Context, accountID, charID int) error
	ResetLocation(ctx context.Context, accountID, charID int) error
}

type HandlerConfig struct {
	Logger  *slog.Logger
	General config.GeneralConfig
}

type Handler struct {
	svc     characterService
	logger  *slog.Logger
	general config.GeneralConfig
}

func NewHandler(svc characterService, cfg HandlerConfig) *Handler {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Handler{svc: svc, logger: logger, general: cfg.General}
}

func (h *Handler) layout() httpx.Layout {
	return httpx.Layout{GeneralConfig: h.general}
}

func (h *Handler) RegisterRoutes(reg *routes.Registry, mux *http.ServeMux) {
	reg.Wrap(mux, "Character.View", "GET /characters/{charID}", http.HandlerFunc(h.showDetail))
	reg.Wrap(mux, "Character.Reset", "POST /characters/{charID}/look", http.HandlerFunc(h.doResetLook))
	reg.Wrap(mux, "Character.Reset", "POST /characters/{charID}/location", http.HandlerFunc(h.doResetLocation))
}
