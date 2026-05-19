package transport

import (
	"errors"
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/features/mob/app"
	"github.com/hayakawakaki/go-racp/internal/features/mob/domain"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
)

func (h *Handler) showList(w http.ResponseWriter, r *http.Request) {
	query := app.ListQuery{
		Page:    httpx.ParsePositiveInt(r.URL.Query().Get("page"), 1),
		PerPage: app.DefaultPerPage,
		Query:   r.URL.Query().Get("q"),
	}

	page, err := h.svc.List(r.Context(), query)
	if errors.Is(err, domain.ErrEmptySnapshot) {
		httpx.RenderHTML(w, r, h.logger, emptyDatabasePage(h.layout()))
		return
	}
	if err != nil {
		h.logger.Error("mob: list", "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	state := ListState{Page: page, Query: query.Query, BaseURL: "/mobs"}
	httpx.RenderHTML(w, r, h.logger, listPage(h.layout(), state))
}
