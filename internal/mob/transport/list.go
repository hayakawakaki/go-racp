package transport

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/hayakawakaki/go-racp/internal/httpx"
	mobapp "github.com/hayakawakaki/go-racp/internal/mob/app"
	"github.com/hayakawakaki/go-racp/internal/mob/domain"
)

func (h *Handler) showList(w http.ResponseWriter, r *http.Request) {
	query := mobapp.ListQuery{
		Page:    parsePositiveInt(r.URL.Query().Get("page"), 1),
		PerPage: mobapp.DefaultPerPage,
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
