package transport

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/hayakawakaki/go-racp/internal/httpx"
	itemapp "github.com/hayakawakaki/go-racp/internal/item/app"
	"github.com/hayakawakaki/go-racp/internal/item/domain"
)

const defaultPerPage = 20

func (h *Handler) showList(w http.ResponseWriter, r *http.Request) {
	query := itemapp.ListQuery{
		Page:    parsePositiveInt(r.URL.Query().Get("page"), 1),
		PerPage: defaultPerPage,
		Query:   r.URL.Query().Get("q"),
	}
	typeName := r.URL.Query().Get("type")
	if typeName != "" {
		if value, ok := domain.ItemTypeFromString(typeName); ok {
			query.Type = value
		}
	}

	page, err := h.svc.List(r.Context(), query)
	if errors.Is(err, domain.ErrEmptySnapshot) {
		httpx.RenderHTML(w, r, h.logger, emptyDatabasePage(h.layout()))

		return
	}
	if err != nil {
		h.logger.Error("item: list", "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)

		return
	}

	state := ListState{Page: page, Query: query.Query, Type: typeName, BaseURL: "/items"}
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
