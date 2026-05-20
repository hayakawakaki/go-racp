package transport

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

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
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{svc: svc, logger: logger}
}

func (h *Handler) RegisterRoutes(reg *routes.Registry, mux *http.ServeMux) {
	reg.Wrap(mux, "Metric.API", "GET /api/v1/metrics/online", http.HandlerFunc(h.online))
	reg.Wrap(mux, "Metric.API", "GET /api/v1/metrics/peaks", http.HandlerFunc(h.peaks))
	reg.Wrap(mux, "Metric.API", "GET /api/v1/metrics/general", http.HandlerFunc(h.general))
}

func (h *Handler) online(w http.ResponseWriter, r *http.Request) {
	snap := h.svc.Online(r.Context())
	h.writeJSON(w, snap)
}

func (h *Handler) peaks(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.Peaks(r.Context())
	if err != nil {
		h.logger.Warn("metric: peaks query failed", "err", err)
		rows = nil
	}
	if rows == nil {
		rows = []domain.PeakRow{}
	}
	h.writeJSON(w, rows)
}

func (h *Handler) general(w http.ResponseWriter, r *http.Request) {
	snap, err := h.svc.General(r.Context())
	if err != nil {
		h.logger.Warn("metric: general query failed", "err", err)
	}
	h.writeJSON(w, snap)
}

func (h *Handler) writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		h.logger.Warn("metric: json encode failed", "err", err)
	}
}
