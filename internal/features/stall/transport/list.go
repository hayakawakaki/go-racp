package transport

import (
	"errors"
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/features/stall/domain"
	"github.com/hayakawakaki/go-racp/internal/features/stall/transport/state"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
)

func (h *Handler) showList(w http.ResponseWriter, r *http.Request) {
	typeName := r.URL.Query().Get("type")
	parsedType := domain.VendorTypeUnknown
	if typeName != "" && typeName != "all" {
		if value, ok := domain.VendorTypeFromString(typeName); ok {
			parsedType = value
		} else {
			typeName = ""
		}
	}
	if typeName == "all" {
		typeName = ""
	}
	itemID := httpx.ParsePositiveInt(r.URL.Query().Get("item"), 0)

	page, err := h.svc.List(r.Context(), domain.ListQuery{
		Type:    parsedType,
		ItemID:  itemID,
		Page:    httpx.ParsePositiveInt(r.URL.Query().Get("page"), 1),
		PerPage: 20,
	})
	if errors.Is(err, domain.ErrSnapshotNotReady) {
		refreshURL := r.URL.RequestURI()
		if httpx.IsHTMX(r) {
			httpx.RenderHTML(w, r, h.logger, h.theme.StallLoadingContent(refreshURL))
			return
		}
		httpx.RenderHTML(w, r, h.logger, h.theme.StallLoadingPage(h.layout(), refreshURL))
		return
	}
	if err != nil {
		h.logger.Error("stall: list", "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	s := state.ListState{Page: page, Type: typeName, ItemID: itemID, BaseURL: "/vendors"}
	if httpx.IsHTMX(r) {
		httpx.RenderHTML(w, r, h.logger, h.theme.StallListContent(s))
		return
	}
	httpx.RenderHTML(w, r, h.logger, h.theme.StallListPage(h.layout(), s))
}
