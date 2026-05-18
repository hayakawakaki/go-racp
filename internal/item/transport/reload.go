package transport

import (
	"errors"
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/httpx"
	"github.com/hayakawakaki/go-racp/internal/refdata"
)

func (h *Handler) doReload(w http.ResponseWriter, r *http.Request) {
	err := h.svc.Reload(r.Context())
	switch {
	case errors.Is(err, refdata.ErrReloadConflict):
		w.WriteHeader(http.StatusConflict)
		httpx.RenderHTML(w, r, h.logger, reloadConflict())
		return
	case err != nil:
		h.logger.Error("item: reload failed", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		httpx.RenderHTML(w, r, h.logger, reloadFailure("Reload failed. Check server logs for details."))
		return
	}
	httpx.RenderHTML(w, r, h.logger, reloadSuccess(h.svc.Status()))
}
