package transport

import (
	"errors"
	"net/http"
	"strconv"

	accountdomain "github.com/hayakawakaki/go-racp/internal/account/domain"
	"github.com/hayakawakaki/go-racp/internal/httpx"
	"github.com/hayakawakaki/go-racp/internal/tickets/domain"
)

func (h *Handler) staffReply(w http.ResponseWriter, r *http.Request) {
	h.staffMutate(w, r, maxReplyFormBytes, func(ticketID int64, staffID int) error {
		return h.svc.StaffReply(r.Context(), staffID, ticketID, r.PostFormValue(fieldBody))
	})
}

func (h *Handler) staffNote(w http.ResponseWriter, r *http.Request) {
	h.staffMutate(w, r, maxNoteFormBytes, func(ticketID int64, staffID int) error {
		return h.svc.StaffNote(r.Context(), staffID, ticketID, r.PostFormValue(fieldBody))
	})
}

func (h *Handler) staffResolve(w http.ResponseWriter, r *http.Request) {
	h.staffMutate(w, r, 0, func(ticketID int64, staffID int) error {
		return h.svc.StaffResolve(r.Context(), staffID, ticketID)
	})
}

func (h *Handler) staffClose(w http.ResponseWriter, r *http.Request) {
	h.staffMutate(w, r, 0, func(ticketID int64, staffID int) error {
		return h.svc.StaffClose(r.Context(), staffID, ticketID)
	})
}

func (h *Handler) staffRecategorize(w http.ResponseWriter, r *http.Request) {
	h.staffMutate(w, r, maxSubjectFormBytes, func(ticketID int64, staffID int) error {
		return h.svc.StaffRecategorize(r.Context(), staffID, ticketID, r.PostFormValue(fieldCategory))
	})
}

func (h *Handler) staffEditSubject(w http.ResponseWriter, r *http.Request) {
	h.staffMutate(w, r, maxSubjectFormBytes, func(ticketID int64, staffID int) error {
		return h.svc.StaffEditSubject(r.Context(), staffID, ticketID, r.PostFormValue(fieldSubject))
	})
}

func (h *Handler) staffMutate(w http.ResponseWriter, r *http.Request, maxBytes int64, action func(ticketID int64, staffID int) error) {
	user, role, ok := h.resolveUser(w, r)
	if !ok {
		return
	}
	ticketID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.Render404(w, r, h.logger, h.layout())
		return
	}

	if !h.parseStaffForm(w, r, maxBytes) {
		return
	}

	if !h.assertCategoryAllowed(w, r, role, ticketID) {
		return
	}

	if actionErr := action(ticketID, user.ID); actionErr != nil {
		h.respondActionError(w, actionErr)
		return
	}

	h.respondStaffMutateSuccess(w, r, role, ticketID)
}

func (h *Handler) parseStaffForm(w http.ResponseWriter, r *http.Request, maxBytes int64) bool {
	if maxBytes <= 0 {
		return true
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return false
	}

	return true
}

func (h *Handler) assertCategoryAllowed(w http.ResponseWriter, r *http.Request, role accountdomain.Role, ticketID int64) bool {
	current, err := h.svc.GetTicketForStaff(r.Context(), ticketID)
	if err != nil {
		if errors.Is(err, domain.ErrTicketNotFound) {
			httpx.Render404(w, r, h.logger, h.layout())
			return false
		}
		h.logger.Error("staffMutate: load", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return false
	}
	if !h.categoryAllowed(role, current.Ticket.Category) {
		httpx.Render404(w, r, h.logger, h.layout())
		return false
	}

	return true
}

func (h *Handler) respondStaffMutateSuccess(w http.ResponseWriter, r *http.Request, role accountdomain.Role, ticketID int64) {
	refreshed, err := h.svc.GetTicketForStaff(r.Context(), ticketID)
	if err != nil {
		h.logger.Error("staffMutate: refetch", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if !h.categoryAllowed(role, refreshed.Ticket.Category) {
		w.Header().Set("HX-Redirect", "/tickets")
		if !httpx.IsHTMX(r) {
			httpx.Redirect(w, r, "/tickets")
		}
		return
	}

	httpx.RenderHTML(w, r, h.logger, Thread(refreshed.Messages, true, refreshed.OtherSeenAt))
}

func (h *Handler) respondActionError(w http.ResponseWriter, err error) {
	var validation *domain.ValidationError
	switch {
	case errors.As(err, &validation):
		http.Error(w, validation.Error(), http.StatusBadRequest)
	case errors.Is(err, domain.ErrTicketTerminal),
		errors.Is(err, domain.ErrSubjectUnchanged),
		errors.Is(err, domain.ErrCategoryUnchanged):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, domain.ErrUnknownCategory):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		h.logger.Error("staffMutate: action", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}
