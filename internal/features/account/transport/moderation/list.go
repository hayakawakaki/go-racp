package moderation

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	app "github.com/hayakawakaki/go-racp/internal/features/account/app/moderation"
	"github.com/hayakawakaki/go-racp/internal/features/account/transport/middleware"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
)

func pageURL(baseURL string, page int, query string) string {
	return fmt.Sprintf("%s?page=%d&q=%s", baseURL, page, url.QueryEscape(query))
}

type ListState struct {
	Now     time.Time
	Query   string
	BaseURL string
	Page    app.UserPage
}

func (h *Handler) showList(w http.ResponseWriter, r *http.Request) {
	query := app.ListQuery{
		Page:    httpx.ParsePositiveInt(r.URL.Query().Get("page"), 1),
		PerPage: 20,
		Query:   r.URL.Query().Get("q"),
	}
	if snap, ok := middleware.SnapshotFromContext(r.Context()); ok && snap != nil {
		query.ExcludeID = snap.UserID
	}
	page, err := h.svc.List(r.Context(), query)
	if err != nil {
		h.logger.Error("users: list failed", "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	state := ListState{Page: page, Query: query.Query, BaseURL: "/users", Now: time.Now()}
	if httpx.IsHTMX(r) {
		httpx.RenderHTML(w, r, h.logger, h.theme.UsersListContent(state))
		return
	}
	httpx.RenderHTML(w, r, h.logger, h.theme.UsersListPage(h.layout(), state))
}
