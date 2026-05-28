package transport

import (
	"errors"
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/features/stall/domain"
	"github.com/hayakawakaki/go-racp/internal/features/stall/transport/state"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
)

const vendorsPerPage = 8

func (h *Handler) showList(w http.ResponseWriter, r *http.Request) {
	itemID := httpx.ParsePositiveInt(r.URL.Query().Get("item"), 0)
	buyingPage := httpx.ParsePositiveInt(r.URL.Query().Get("buying_page"), 1)
	sellingPage := httpx.ParsePositiveInt(r.URL.Query().Get("selling_page"), 1)

	buying, err := h.svc.List(r.Context(), domain.ListQuery{
		Type:    domain.VendorTypeBuying,
		ItemID:  itemID,
		Page:    buyingPage,
		PerPage: vendorsPerPage,
	})
	if errors.Is(err, domain.ErrSnapshotNotReady) {
		h.renderLoading(w, r)
		return
	}
	if err != nil {
		h.logger.Error("stall: list buying", "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	selling, err := h.svc.List(r.Context(), domain.ListQuery{
		Type:    domain.VendorTypeSelling,
		ItemID:  itemID,
		Page:    sellingPage,
		PerPage: vendorsPerPage,
	})
	if errors.Is(err, domain.ErrSnapshotNotReady) {
		h.renderLoading(w, r)
		return
	}
	if err != nil {
		h.logger.Error("stall: list selling", "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	s := state.ListState{
		BuyingPage:  buying,
		SellingPage: selling,
		ItemID:      itemID,
		BaseURL:     "/vendors",
	}
	if httpx.IsHTMX(r) {
		httpx.RenderHTML(w, r, h.logger, h.theme.StallListContent(s))
		return
	}
	httpx.RenderHTML(w, r, h.logger, h.theme.StallListPage(h.layout(), s))
}

func (h *Handler) renderLoading(w http.ResponseWriter, r *http.Request) {
	refreshURL := r.URL.RequestURI()
	if httpx.IsHTMX(r) {
		httpx.RenderHTML(w, r, h.logger, h.theme.StallLoadingContent(refreshURL))
		return
	}
	httpx.RenderHTML(w, r, h.logger, h.theme.StallLoadingPage(h.layout(), refreshURL))
}
