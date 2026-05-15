package transport

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/hayakawakaki/go-racp/internal/httpx"
	"github.com/hayakawakaki/go-racp/internal/tickets/domain"
)

func (h *Handler) playerDetail(w http.ResponseWriter, r *http.Request) {
	user, _, ok := h.currentUser(r)
	if !ok {
		httpx.Redirect(w, r, "/login")
		return
	}
	ticketID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.Render404(w, r, h.logger, h.layout())
		return
	}

	detail, err := h.svc.GetTicketForPlayer(r.Context(), user.ID, ticketID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrTicketNotFound), errors.Is(err, domain.ErrNotTicketOwner):
			httpx.Render404(w, r, h.logger, h.layout())
		default:
			h.logger.Error("playerDetail", "err", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	go h.deferMarkViewed(r, user.ID, ticketID)

	httpx.RenderHTML(w, r, h.logger, playerDetailPage(h.layout(), PlayerDetailState{Detail: detail}))
}

func (h *Handler) deferMarkViewed(r *http.Request, accountID int, ticketID int64) {
	select {
	case <-r.Context().Done():
		return
	default:
	}
	h.svc.MarkViewed(r.Context(), accountID, ticketID)
}

func (h *Handler) playerReply(w http.ResponseWriter, r *http.Request) {
	user, _, ok := h.currentUser(r)
	if !ok {
		httpx.Redirect(w, r, "/login")
		return
	}
	ticketID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.Render404(w, r, h.logger, h.layout())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxReplyFormBytes)
	if parseErr := r.ParseForm(); parseErr != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	if replyErr := h.svc.PlayerReply(r.Context(), user.ID, ticketID, r.PostFormValue(fieldBody)); replyErr != nil {
		switch {
		case errors.Is(replyErr, domain.ErrTicketNotFound), errors.Is(replyErr, domain.ErrNotTicketOwner):
			httpx.Render404(w, r, h.logger, h.layout())
		case errors.Is(replyErr, domain.ErrTicketTerminal), errors.Is(replyErr, domain.ErrPlayerCannotReply):
			http.Error(w, "cannot reply", http.StatusConflict)
		default:
			h.logger.Error("playerReply", "err", replyErr)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	detail, err := h.svc.GetTicketForPlayer(r.Context(), user.ID, ticketID)
	if err != nil {
		h.logger.Error("playerReply: refetch", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	httpx.RenderHTML(w, r, h.logger, Thread(detail.Messages, false))
}
