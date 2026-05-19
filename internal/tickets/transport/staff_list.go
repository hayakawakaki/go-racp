package transport

import (
	"net/http"
	"strconv"

	accdomain "github.com/hayakawakaki/go-racp/internal/features/account/domain"
	"github.com/hayakawakaki/go-racp/internal/httpx"
	"github.com/hayakawakaki/go-racp/internal/tickets/domain"
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
	offset := (page - 1) * domain.PageSize

	items, total, err := h.svc.ListForStaff(r.Context(), user.ID, tab, allowed, offset, domain.PageSize)
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

func parseTab(s string) domain.StaffTab {
	switch s {
	case "active":
		return domain.TabActive
	case "terminal":
		return domain.TabTerminal
	default:
		return domain.TabOpenNoResponse
	}
}
