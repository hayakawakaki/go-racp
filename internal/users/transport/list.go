package transport

import (
	"net/http"
	"strconv"
	"time"

	"github.com/hayakawakaki/go-racp/internal/httpx"
	"github.com/hayakawakaki/go-racp/internal/users/app"
)

type listState struct {
	Now     time.Time
	Query   string
	BaseURL string
	Page    app.UserPage
}

func (h *Handler) showList(w http.ResponseWriter, r *http.Request) {
	query := app.ListQuery{
		Page:    parsePositiveInt(r.URL.Query().Get("page"), 1),
		PerPage: 20,
		Query:   r.URL.Query().Get("q"),
	}
	page, err := h.svc.List(r.Context(), query)
	if err != nil {
		h.logger.Error("users: list failed", "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)

		return
	}

	state := listState{Page: page, Query: query.Query, BaseURL: "/admin/users", Now: time.Now()}
	if httpx.IsHTMX(r) {
		httpx.RenderHTML(w, r, h.logger, listContent(state))

		return
	}
	httpx.RenderHTML(w, r, h.logger, listPage(h.layout(), state))
}

func parsePositiveInt(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return fallback
	}

	return parsed
}
