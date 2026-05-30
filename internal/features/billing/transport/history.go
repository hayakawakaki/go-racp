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

	st := state.PurchaseHistoryState{Currency: h.currency, Location: h.general.Location()}
	if snapshot, ok := middleware.SnapshotFromContext(r.Context()); ok {
		purchases, err := h.svc.HistoryByAccount(r.Context(), snapshot.UserID, historyPageSize)
		if err != nil {
			h.logger.Error("billing: history by account", "err", err)
		} else {
			st.Purchases = purchases
		}
	}

	if httpx.IsHTMX(r) {
		httpx.RenderHTML(w, r, h.logger, h.theme.PurchaseHistoryContent(st))
		return
	}

	httpx.RenderHTML(w, r, h.logger, h.theme.PurchaseHistoryPage(h.layout(), st))
}
