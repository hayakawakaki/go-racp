package transport

import (
	"errors"
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/features/guild/domain"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
)

func (h *Handler) showEmblem(w http.ResponseWriter, r *http.Request) {
	id := httpx.ParsePositiveInt(r.PathValue("id"), 0)
	if id == 0 {
		http.NotFound(w, r)
		return
	}

	data, mime, err := h.svc.GetEmblem(r.Context(), id)
	switch {
	case errors.Is(err, domain.ErrGuildNotFound),
		errors.Is(err, domain.ErrEmblemEmpty),
		errors.Is(err, domain.ErrEmblemUnknownFormat):
		http.NotFound(w, r)
		return
	case err != nil:
		h.logger.Error("guild: emblem failed", "id", id, "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", mime)
	_, _ = w.Write(data) //nolint:gosec // image blob, not HTML
}
