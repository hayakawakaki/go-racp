package transport

import (
	"net/http"
	"strconv"

	accdomain "github.com/hayakawakaki/go-racp/internal/features/account/domain"
	domain2 "github.com/hayakawakaki/go-racp/internal/features/tickets/domain"
	"github.com/hayakawakaki/go-racp/internal/httpx"
)

func (h *Handler) staffList(w http.ResponseWriter, r *http.Request) {
	user, role, ok := h.resolveUser(w, r)
	if !ok {
		return
	}
	allowed := h.svc.Categories().AllowedForRole(role.Name, role == accdomain.RoleAdmin)

	tab := parseTab(r.URL.Query().Get("tab"))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * domain2.PageSize

	items, total, err := h.svc.ListForStaff(r.Context(), user.ID, tab, allowed, offset, domain2.PageSize)
	if err != nil {
		h.logger.Error("staffList", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	state := StaffListState{
		Items:        items,
		Tab:          tab,
		Page:         page,
		Total:        total,
		PollInterval: h.poll,
	}
	if httpx.IsHTMX(r) {
		httpx.RenderHTML(w, r, h.logger, staffListBody(state))
		return
	}
	httpx.RenderHTML(w, r, h.logger, staffListPage(h.layout(), state))
}

func parseTab(s string) domain2.StaffTab {
	switch s {
	case "active":
		return domain2.TabActive
	case "terminal":
		return domain2.TabTerminal
	default:
		return domain2.TabOpenNoResponse
	}
}
