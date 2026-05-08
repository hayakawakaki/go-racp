package transport

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/auth/app"
	authtransport "github.com/hayakawakaki/go-racp/internal/auth/transport"
	"github.com/hayakawakaki/go-racp/internal/httpx"
)

type userService interface {
	GetByID(ctx context.Context, id int) (*app.GetDTO, error)
}

type Handler struct {
	userSvc userService
	logger  *slog.Logger
}

func NewHandler(userSvc userService, logger *slog.Logger) *Handler {
	return &Handler{userSvc: userSvc, logger: logger}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, wrap func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("GET /{$}", wrap(h.show))
}

func (h *Handler) show(w http.ResponseWriter, r *http.Request) {
	state := HomeState{}
	if sess, ok := authtransport.SessionFromContext(r.Context()); ok {
		user, err := h.userSvc.GetByID(r.Context(), sess.UserID)
		if err != nil {
			h.logger.Error("home: fetch user", "err", err, "userID", sess.UserID)
		} else {
			state.Username = user.Username
		}
	}
	httpx.RenderHTML(w, r, h.logger, homePage(state))
}
