package transport

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hayakawakaki/go-racp/internal/features/stall/domain"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
)

type listState struct {
	Type    string
	BaseURL string
	Page    domain.Page
	ItemID  int
}

func pageURL(baseURL string, page int, typeName string, itemID int) string {
	values := url.Values{}
	values.Set("page", fmt.Sprintf("%d", page))
	if typeName != "" {
		values.Set("type", typeName)
	}
	if itemID > 0 {
		values.Set("item", fmt.Sprintf("%d", itemID))
	}

	return baseURL + "?" + values.Encode()
}

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
			httpx.RenderHTML(w, r, h.logger, loadingContent(refreshURL))
			return
		}
		httpx.RenderHTML(w, r, h.logger, loadingPage(h.layout(), refreshURL))
		return
	}
	if err != nil {
		h.logger.Error("stall: list", "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	state := listState{Page: page, Type: typeName, ItemID: itemID, BaseURL: "/vendors"}
	if httpx.IsHTMX(r) {
		httpx.RenderHTML(w, r, h.logger, listContent(state))
		return
	}
	httpx.RenderHTML(w, r, h.logger, listPage(h.layout(), state))
}
