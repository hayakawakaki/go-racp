package transport

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	app "github.com/hayakawakaki/go-racp/internal/features/account/app/self"
	"github.com/hayakawakaki/go-racp/internal/features/account/transport/middleware"
	"github.com/hayakawakaki/go-racp/internal/features/home/transport/state"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
	"github.com/hayakawakaki/go-racp/server/config"
)

type userService interface {
	GetByID(ctx context.Context, id int) (*app.GetDTO, error)
}

type Renderer interface {
	HomePage(layout httpx.Layout, state state.HomeState) templ.Component
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
	userSvc userService
	theme   Renderer
	logger  *slog.Logger
}

func NewHandler(userSvc userService, cfg HandlerConfig) *Handler {
	return &Handler{
		userSvc: userSvc,
		logger:  cfg.Logger,
		general: cfg.General,
		theme:   cfg.Theme,
	}
}

func (h *Handler) layout() httpx.Layout {
	return httpx.Layout{GeneralConfig: h.general}
}

func (h *Handler) RegisterRoutes(reg *routes.Registry, mux *http.ServeMux) {
	reg.Wrap(mux, "Home.View", "GET /{$}", http.HandlerFunc(h.show))
}

func (h *Handler) show(w http.ResponseWriter, r *http.Request) {
	s := state.HomeState{}
	if sess, ok := middleware.SessionFromContext(r.Context()); ok {
		user, err := h.userSvc.GetByID(r.Context(), sess.UserID)
		if err != nil {
			h.logger.Error("home: fetch user", "err", err, "userID", sess.UserID)
		} else {
			s.Username = user.Username
		}
	}

	httpx.RenderHTML(w, r, h.logger, h.theme.HomePage(h.layout(), s))
}
