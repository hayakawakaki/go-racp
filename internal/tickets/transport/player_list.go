package transport

import (
	"net/http"
	"strconv"

	"github.com/hayakawakaki/go-racp/internal/httpx"
	"github.com/hayakawakaki/go-racp/internal/tickets/domain"
)

func (h *Handler) playerList(w http.ResponseWriter, r *http.Request) {
	user, _, ok := h.currentUser(r)
	if !ok {
		httpx.Redirect(w, r, "/login")
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
