package moderation

import (
	"net/http"
	"time"

	app "github.com/hayakawakaki/go-racp/internal/features/account/app/moderation"
	"github.com/hayakawakaki/go-racp/internal/features/account/transport/middleware"
	"github.com/hayakawakaki/go-racp/internal/features/account/transport/moderation/state"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
)

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

	s := state.ListState{Page: page, Query: query.Query, BaseURL: "/users", Now: time.Now()}
	if httpx.IsHTMX(r) {
		httpx.RenderHTML(w, r, h.logger, h.theme.UsersListContent(s))
		return
	}
	httpx.RenderHTML(w, r, h.logger, h.theme.UsersListPage(h.layout(), s))
}
