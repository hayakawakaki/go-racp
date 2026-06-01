package transport

import (
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/features/account/transport/middleware"
	"github.com/hayakawakaki/go-racp/internal/features/billing/transport/state"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
)

func (h *Handler) showHistory(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil {
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		return
	}

	st := h.purchaseHistoryState(r)
	if httpx.IsHTMX(r) {
		httpx.RenderHTML(w, r, h.logger, h.theme.PurchaseHistoryContent(st))
		return
	}

	httpx.RenderHTML(w, r, h.logger, h.theme.PurchaseHistoryPage(h.layout(), st))
}

func (h *Handler) showHistorySummary(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil {
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		return
	}

	st := h.purchaseHistoryState(r)

	httpx.RenderHTML(w, r, h.logger, h.theme.PurchaseHistorySummary(st))
}

func (h *Handler) purchaseHistoryState(r *http.Request) state.PurchaseHistoryState {
	st := state.PurchaseHistoryState{Currency: h.currency, Location: h.general.Location()}
	snapshot, ok := middleware.SnapshotFromContext(r.Context())
	if !ok {
		return st
	}

	purchases, err := h.svc.HistoryByAccount(r.Context(), snapshot.UserID, historyPageSize)
	if err != nil {
		h.logger.Error("billing: history by account", "err", err)
		return st
	}

	st.Purchases = purchases

	return st
}
