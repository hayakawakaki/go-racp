package transport

import (
	"net/http"
	"strconv"

	"github.com/hayakawakaki/go-racp/internal/features/tickets/domain"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
)

func (h *Handler) playerList(w http.ResponseWriter, r *http.Request) {
	user, _, ok := h.resolveUser(w, r)
	if !ok {
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * domain.PageSize

	items, total, err := h.svc.ListForPlayer(r.Context(), user.ID, offset, domain.PageSize)
	if err != nil {
		h.logger.Error("playerList", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	httpx.RenderHTML(w, r, h.logger, playerListPage(h.layout(), PlayerListState{
		Items: items,
		Page:  page,
		Total: total,
	}))
}
