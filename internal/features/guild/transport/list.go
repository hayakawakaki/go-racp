package transport

import (
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/features/guild/app"
	"github.com/hayakawakaki/go-racp/internal/features/guild/transport/state"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
)

func (h *Handler) showList(w http.ResponseWriter, r *http.Request) {
	query := app.ListQuery{
		Page:    httpx.ParsePositiveInt(r.URL.Query().Get("page"), 1),
		PerPage: 20,
		Query:   r.URL.Query().Get("q"),
	}
	page, err := h.svc.List(r.Context(), query)
	if err != nil {
		h.logger.Error("guild: list failed", "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	s := state.ListState{Page: page, Query: query.Query, BaseURL: "/guilds"}
	if httpx.IsHTMX(r) {
		httpx.RenderHTML(w, r, h.logger, h.theme.GuildListContent(s))
		return
	}
	httpx.RenderHTML(w, r, h.logger, h.theme.GuildListPage(h.layout(), s))
}
