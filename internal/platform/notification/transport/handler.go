package transport

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/hayakawakaki/go-racp/internal/features/account/transport/middleware"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
	notificationapp "github.com/hayakawakaki/go-racp/internal/platform/notification/app"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
)

type Handler struct {
	svc    *notificationapp.Service
	logger *slog.Logger
}

func NewHandler(svc *notificationapp.Service, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, logger: logger}
}

func (h *Handler) RegisterRoutes(reg *routes.Registry, mux *http.ServeMux) {
	reg.Wrap(mux, "Notification.View", "GET /notifications/menu", http.HandlerFunc(h.menu))
	reg.Wrap(mux, "Notification.View", "GET /notifications/unread-count", http.HandlerFunc(h.unreadCount))
	reg.Wrap(mux, "Notification.View", "GET /notifications/stream", http.HandlerFunc(h.stream))
	reg.Wrap(mux, "Notification.View", "POST /notifications/{id}/read", http.HandlerFunc(h.markRead))
	reg.Wrap(mux, "Notification.View", "POST /notifications/read-all", http.HandlerFunc(h.markAllRead))
}

func (h *Handler) accountID(r *http.Request) (int, bool) {
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		return 0, false
	}

	return session.UserID, true
}

func (h *Handler) menu(w http.ResponseWriter, r *http.Request) {
	accountID, ok := h.accountID(r)
	if !ok {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	items, err := h.svc.Recent(r.Context(), accountID)
	if err != nil {
		h.logger.Error("notification: recent", "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	httpx.RenderHTML(w, r, h.logger, Menu(items))
}

func (h *Handler) unreadCount(w http.ResponseWriter, r *http.Request) {
	accountID, ok := h.accountID(r)
	if !ok {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	count, err := h.svc.UnreadCount(r.Context(), accountID)
	if err != nil {
		h.logger.Error("notification: unread count", "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if err := httpx.WriteJSON(w, http.StatusOK, map[string]int{"count": count}); err != nil {
		h.logger.Error("notification: write count", "err", err)
	}
}

func (h *Handler) markRead(w http.ResponseWriter, r *http.Request) {
	accountID, ok := h.accountID(r)
	if !ok {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if err := h.svc.MarkRead(r.Context(), accountID, id); err != nil {
		h.logger.Error("notification: mark read", "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) markAllRead(w http.ResponseWriter, r *http.Request) {
	accountID, ok := h.accountID(r)
	if !ok {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	if err := h.svc.MarkAllRead(r.Context(), accountID); err != nil {
		h.logger.Error("notification: mark all read", "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	items, err := h.svc.Recent(r.Context(), accountID)
	if err != nil {
		h.logger.Error("notification: recent", "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	httpx.RenderHTML(w, r, h.logger, Menu(items))
}
