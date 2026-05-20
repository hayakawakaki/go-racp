package transport

import (
	"errors"
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/features/stall/domain"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
)

type stallItemRow struct {
	ItemName string
	Aegis    string
	ItemID   int
	Amount   int
	Price    int
}

type stallState struct {
	Items  []stallItemRow
	Vendor domain.Vendor
}

func (h *Handler) showStallItems(w http.ResponseWriter, r *http.Request) {
	vendorType, ok := domain.VendorTypeFromString(r.PathValue("type"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	id := httpx.ParsePositiveInt(r.PathValue("id"), 0)
	if id == 0 {
		http.NotFound(w, r)
		return
	}

	v, err := h.svc.Get(r.Context(), domain.VendorKey{Type: vendorType, ID: id})
	if h.resolveStallError(w, r, err, vendorType, id) {
		return
	}

	httpx.RenderHTML(w, r, h.logger, vendingBox(stallState{Vendor: v, Items: h.buildItemRows(r, v.Items)}))
}

func (h *Handler) resolveStallError(w http.ResponseWriter, r *http.Request, err error, vendorType domain.VendorType, id int) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, domain.ErrSnapshotNotReady) {
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		return true
	}
	if errors.Is(err, domain.ErrVendorNotFound) {
		http.NotFound(w, r)
		return true
	}
	h.logger.Error("stall: items", "err", err, "type", vendorType.String(), "id", id)
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)

	return true
}

func (h *Handler) buildItemRows(r *http.Request, items []domain.VendorItem) []stallItemRow {
	rows := make([]stallItemRow, 0, len(items))
	for _, item := range items {
		row := stallItemRow{ItemID: item.ItemID, Amount: item.Amount, Price: item.Price}
		if h.itemLookup != nil {
			if name, aegis, ok := h.resolveItemName(r, item.ItemID); ok {
				row.ItemName = name
				row.Aegis = aegis
			}
		}
		rows = append(rows, row)
	}

	return rows
}

func (h *Handler) resolveItemName(r *http.Request, itemID int) (name, aegis string, ok bool) {
	lookedUp, err := h.itemLookup.Get(r.Context(), itemID)
	if err != nil || lookedUp == nil {
		return "", "", false
	}

	return lookedUp.ClientName, lookedUp.AegisName, true
}
