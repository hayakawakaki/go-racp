package transport

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/account/app"
	"github.com/hayakawakaki/go-racp/internal/account/transport/middleware"
	"github.com/hayakawakaki/go-racp/internal/httpx"
	"github.com/hayakawakaki/go-racp/server/config"
)

type userService interface {
	GetByID(ctx context.Context, id int) (*app.GetDTO, error)
}

type HandlerConfig struct {
	Logger  *slog.Logger
	General config.GeneralConfig
}

type Handler struct {
	userSvc userService
	logger  *slog.Logger
	general config.GeneralConfig
}

func NewHandler(userSvc userService, cfg HandlerConfig) *Handler {
	return &Handler{
		userSvc: userSvc,
		logger:  cfg.Logger,
		general: cfg.General,
	}
}

func (h *Handler) layout() httpx.Layout {
	return httpx.Layout{GeneralConfig: h.general}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, wrap func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("GET /{$}", wrap(h.show))
}

func (h *Handler) show(w http.ResponseWriter, r *http.Request) {
	state := HomeState{}
	if sess, ok := middleware.SessionFromContext(r.Context()); ok {
		user, err := h.userSvc.GetByID(r.Context(), sess.UserID)
		if err != nil {
			h.logger.Error("home: fetch user", "err", err, "userID", sess.UserID)
		} else {
			state.Username = user.Username
		}
	}

	httpx.RenderHTML(w, r, h.logger, homePage(h.layout(), state))
}
