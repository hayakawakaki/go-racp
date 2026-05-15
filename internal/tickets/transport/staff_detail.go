package transport

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/hayakawakaki/go-racp/internal/httpx"
	"github.com/hayakawakaki/go-racp/internal/tickets/domain"
)

func (h *Handler) staffDetail(w http.ResponseWriter, r *http.Request) {
	user, role, ok := h.resolveUser(w, r)
	if !ok {
		return
	}
	ticketID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.Render404(w, r, h.logger, h.layout())
		return
	}

	detail, err := h.svc.GetTicketForStaff(r.Context(), ticketID)
	if err != nil {
		if errors.Is(err, domain.ErrTicketNotFound) {
			httpx.Render404(w, r, h.logger, h.layout())
			return
		}
		h.logger.Error("staffDetail", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !h.categoryAllowed(role, detail.Ticket.Category) {
		httpx.Render404(w, r, h.logger, h.layout())
		return
	}

	go h.deferMarkViewed(context.WithoutCancel(r.Context()), user.ID, ticketID)

	httpx.RenderHTML(w, r, h.logger, staffDetailPage(h.layout(), StaffDetailState{
		Detail:     detail,
		Categories: h.svc.Categories().All(),
	}))
}
