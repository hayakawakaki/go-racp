package transport

import (
	"cmp"
	"context"
	"log/slog"
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
	"github.com/hayakawakaki/go-racp/internal/platform/metric/domain"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
)

type reader interface {
	Online(ctx context.Context) domain.OnlineSnapshot
	Peaks(ctx context.Context) ([]domain.PeakRow, error)
	General(ctx context.Context) (domain.GeneralSnapshot, error)
}

type Handler struct {
	svc    reader
	logger *slog.Logger
}

func NewHandler(svc reader, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, logger: cmp.Or(logger, slog.Default())}
}

func (h *Handler) RegisterRoutes(reg *routes.Registry, mux *http.ServeMux) {
	reg.Wrap(mux, "Metric.API", "GET /api/v1/metrics/online", http.HandlerFunc(h.online))
	reg.Wrap(mux, "Metric.API", "GET /api/v1/metrics/peaks", http.HandlerFunc(h.peaks))
	reg.Wrap(mux, "Metric.API", "GET /api/v1/metrics/general", http.HandlerFunc(h.general))
}

func (h *Handler) online(w http.ResponseWriter, r *http.Request) {
	snap := h.svc.Online(r.Context())
	if err := httpx.WriteJSON(w, http.StatusOK, snap); err != nil {
		h.logger.Warn("metric: online encode failed", "err", err)
	}
}

func (h *Handler) peaks(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.Peaks(r.Context())
	if err != nil {
		h.logger.Error("metric: peaks query failed", "err", err)
		h.writeError(w, http.StatusInternalServerError, "peaks query failed")
		return
	}
	if rows == nil {
		rows = []domain.PeakRow{}
	}
	if err := httpx.WriteJSON(w, http.StatusOK, rows); err != nil {
		h.logger.Warn("metric: peaks encode failed", "err", err)
	}
}

func (h *Handler) general(w http.ResponseWriter, r *http.Request) {
	snap, err := h.svc.General(r.Context())
	if err != nil {
		h.logger.Error("metric: general query failed", "err", err)
		h.writeError(w, http.StatusInternalServerError, "general query failed")
		return
	}
	if err := httpx.WriteJSON(w, http.StatusOK, snap); err != nil {
		h.logger.Warn("metric: general encode failed", "err", err)
	}
}

func (h *Handler) writeError(w http.ResponseWriter, status int, message string) {
	if err := httpx.WriteJSON(w, status, map[string]string{"error": message}); err != nil {
		h.logger.Warn("metric: error response encode failed", "err", err)
	}
}
