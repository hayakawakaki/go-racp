package transport

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/hayakawakaki/go-racp/internal/features/guild/app"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
)

type listState struct {
	Query   string
	BaseURL string
	Page    app.GuildPage
}

func pageURL(baseURL string, page int, query string) string {
	return fmt.Sprintf("%s?page=%d&q=%s", baseURL, page, url.QueryEscape(query))
}

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

	state := listState{Page: page, Query: query.Query, BaseURL: "/guilds"}
	if httpx.IsHTMX(r) {
		httpx.RenderHTML(w, r, h.logger, listContent(state))
		return
	}
	httpx.RenderHTML(w, r, h.logger, listPage(h.layout(), state))
}
