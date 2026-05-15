package transport

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/hayakawakaki/go-racp/internal/httpx"
	"github.com/hayakawakaki/go-racp/internal/tickets/domain"
)

func (h *Handler) playerNewForm(w http.ResponseWriter, r *http.Request) {
	_, _, ok := h.resolveUser(w, r)
	if !ok {
		return
	}
	httpx.RenderHTML(w, r, h.logger, playerNewPage(h.layout(), PlayerNewState{
		Categories: h.svc.Categories().All(),
	}))
}

func (h *Handler) playerCreate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := h.resolveUser(w, r)
	if !ok {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxOpenFormBytes)
	if err := r.ParseForm(); err != nil {
		h.renderNewWithError(w, r, PlayerNewState{FormError: "Invalid form data."})
		return
	}

	submission := PlayerNewState{
		Category: r.PostFormValue(fieldCategory),
		Subject:  r.PostFormValue(fieldSubject),
		Body:     r.PostFormValue(fieldBody),
	}

	id, err := h.svc.OpenTicket(r.Context(), user.ID, submission.Category, submission.Subject, submission.Body)
	if err != nil {
		state := submission
		var validation *domain.ValidationError
		switch {
		case errors.As(err, &validation):
			state.Errors = validation.Fields
		case errors.Is(err, domain.ErrTooManyOpenTickets):
			state.FormError = "You have too many open tickets."
		case errors.Is(err, domain.ErrTicketCooldown):
			state.FormError = "Please wait before opening another ticket."
		case errors.Is(err, domain.ErrUnknownCategory):
			state.FormError = "Unknown category."
		default:
			h.logger.Error("playerCreate", "err", err)
			state.FormError = "Something went wrong."
		}
		h.renderNewWithError(w, r, state)
		return
	}

	httpx.Redirect(w, r, "/tickets/"+strconv.FormatInt(id, 10))
}

func (h *Handler) renderNewWithError(w http.ResponseWriter, r *http.Request, state PlayerNewState) {
	state.Categories = h.svc.Categories().All()
	httpx.RenderHTML(w, r, h.logger, playerNewPage(h.layout(), state))
}
