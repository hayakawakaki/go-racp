package transport

import (
	"errors"
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/features/guild/domain"
	"github.com/hayakawakaki/go-racp/internal/features/guild/transport/state"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
)

func (h *Handler) showDetail(w http.ResponseWriter, r *http.Request) {
	id := httpx.ParsePositiveInt(r.PathValue("id"), 0)
	if id == 0 {
		httpx.Render404(w, r, h.logger, h.layout())
		return
	}

	detail, err := h.svc.Get(r.Context(), id)
	if errors.Is(err, domain.ErrGuildNotFound) {
		httpx.Render404(w, r, h.logger, h.layout())
		return
	}
	if err != nil {
		h.logger.Error("guild: detail failed", "id", id, "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	s := state.DetailState{Detail: detail}
	if httpx.IsHTMX(r) {
		httpx.RenderHTML(w, r, h.logger, h.theme.GuildDetailContent(s))
		return
	}
	httpx.RenderHTML(w, r, h.logger, h.theme.GuildDetailPage(h.layout(), detail.Guild.Name, s))
}
