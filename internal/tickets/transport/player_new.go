package transport

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/hayakawakaki/go-racp/internal/httpx"
	"github.com/hayakawakaki/go-racp/internal/tickets/domain"
)

func (h *Handler) playerNewForm(w http.ResponseWriter, r *http.Request) {
	_, _, ok := h.currentUser(r)
	if !ok {
		httpx.Redirect(w, r, "/login")
		return
	}
	httpx.RenderHTML(w, r, h.logger, playerNewPage(h.layout(), PlayerNewState{
		Categories: h.svc.Categories().All(),
	}))
}

func (h *Handler) playerCreate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := h.currentUser(r)
	if !ok {
		httpx.Redirect(w, r, "/login")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxOpenFormBytes)
	if err := r.ParseForm(); err != nil {
		h.renderNewWithError(w, r, "Invalid form data.", nil)
		return
	}

	category := r.PostFormValue(fieldCategory)
	subject := r.PostFormValue(fieldSubject)
	body := r.PostFormValue(fieldBody)

	id, err := h.svc.OpenTicket(r.Context(), user.ID, category, subject, body)
	if err != nil {
		var validation *domain.ValidationError
		switch {
		case errors.As(err, &validation):
			h.renderNewWithError(w, r, "", validation.Fields)
		case errors.Is(err, domain.ErrTooManyOpenTickets):
			h.renderNewWithError(w, r, "You have too many open tickets.", nil)
		case errors.Is(err, domain.ErrTicketCooldown):
			h.renderNewWithError(w, r, "Please wait before opening another ticket.", nil)
		case errors.Is(err, domain.ErrUnknownCategory):
			h.renderNewWithError(w, r, "Unknown category.", nil)
		default:
			h.logger.Error("playerCreate", "err", err)
			h.renderNewWithError(w, r, "Something went wrong.", nil)
		}
		return
	}

	httpx.Redirect(w, r, "/tickets/"+strconv.FormatInt(id, 10))
}

func (h *Handler) renderNewWithError(w http.ResponseWriter, r *http.Request, formError string, fieldErrors domain.FieldErrors) {
	httpx.RenderHTML(w, r, h.logger, playerNewPage(h.layout(), PlayerNewState{
		Categories: h.svc.Categories().All(),
		FormError:  formError,
		Errors:     fieldErrors,
	}))
}
