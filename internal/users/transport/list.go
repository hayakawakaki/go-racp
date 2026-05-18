package transport

import (
	"net/http"
	"time"

	"github.com/hayakawakaki/go-racp/internal/account/transport/middleware"
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

	state := listState{Page: page, Query: query.Query, BaseURL: "/admin/users", Now: time.Now()}
	if httpx.IsHTMX(r) {
		httpx.RenderHTML(w, r, h.logger, listContent(state))

		return
	}
	httpx.RenderHTML(w, r, h.logger, listPage(h.layout(), state))
}
