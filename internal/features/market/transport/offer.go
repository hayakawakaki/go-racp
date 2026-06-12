package transport

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/hayakawakaki/go-racp/internal/features/account/transport/middleware"
	"github.com/hayakawakaki/go-racp/internal/features/market/app"
	"github.com/hayakawakaki/go-racp/internal/features/market/domain"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
)

func (h *Handler) registerOfferRoutes(reg *routes.Registry, mux *http.ServeMux) {
	reg.Wrap(mux, "Market.View", "GET /market/listings", http.HandlerFunc(h.browse))
	reg.Wrap(mux, "Market.View", "GET /market/listings/mine", http.HandlerFunc(h.mine))
	reg.Wrap(mux, "Market.Trade", "POST /market/listings", http.HandlerFunc(h.create))
	reg.Wrap(mux, "Market.Trade", "POST /market/listings/{id}/take", http.HandlerFunc(h.take))
	reg.Wrap(mux, "Market.Trade", "POST /market/listings/{id}/cancel", http.HandlerFunc(h.cancel))
}

func (h *Handler) browse(w http.ResponseWriter, r *http.Request) {
	kind, _ := strconv.Atoi(r.URL.Query().Get("kind"))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	const perPage = 30

	rows, total, err := h.offers.Browse(r.Context(), kind, perPage, (page-1)*perPage)
	if err != nil {
		h.logger.Error("market: browse", "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if err := httpx.WriteJSON(w, http.StatusOK, map[string]any{"listings": rows, "total": total, "page": page}); err != nil {
		h.logger.Error("market: write browse", "err", err)
	}
}

func (h *Handler) mine(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	const perPage = 30

	rows, total, err := h.offers.BySeller(r.Context(), session.UserID, perPage, (page-1)*perPage)
	if err != nil {
		h.logger.Error("market: mine", "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if err := httpx.WriteJSON(w, http.StatusOK, map[string]any{"listings": rows, "total": total, "page": page}); err != nil {
		h.logger.Error("market: write mine", "err", err)
	}
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	in := app.CreateInput{
		SellerAccountID: session.UserID,
		Kind:            atoiForm(r, "kind"),
		GiveStashItemID: atoi64Form(r, "give_stash_item_id"),
		GiveUnitAmount:  atoiForm(r, "give_unit_amount"),
		GiveZeny:        atoi64Form(r, "give_zeny"),
		GiveCashpoint:   atoiForm(r, "give_cashpoint"),
		WantNameID:      atoiForm(r, "want_nameid"),
		WantUnitAmount:  atoiForm(r, "want_unit_amount"),
		WantZeny:        atoi64Form(r, "want_zeny"),
		WantCashpoint:   atoiForm(r, "want_cashpoint"),
		Quantity:        atoiForm(r, "quantity"),
	}

	id, err := h.offers.Create(r.Context(), in)
	if err != nil {
		h.writeOfferError(w, r, err)
		return
	}

	if err := httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": id}); err != nil {
		h.logger.Error("market: write create", "err", err)
	}
}

func (h *Handler) take(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	if parseErr := r.ParseForm(); parseErr != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	takeErr := h.offers.Take(r.Context(), app.TakeInput{
		ListingID:        id,
		TakerAccountID:   session.UserID,
		Units:            atoiForm(r, "units"),
		TakerStashItemID: atoi64Form(r, "taker_stash_item_id"),
	})
	if takeErr != nil {
		h.writeOfferError(w, r, takeErr)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if cancelErr := h.offers.Cancel(r.Context(), id, session.UserID); cancelErr != nil {
		h.writeOfferError(w, r, cancelErr)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) writeOfferError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusBadRequest
	switch {
	case errors.Is(err, domain.ErrListingNotFound):
		status = http.StatusNotFound
	case errors.Is(err, domain.ErrStorageUnlocked), errors.Is(err, domain.ErrInsufficientFunds), errors.Is(err, domain.ErrStorageFull):
		status = http.StatusConflict
	}
	h.logger.Warn("market: offer rejected", "err", err, "path", r.URL.Path)
	_ = httpx.WriteJSONError(w, status, err.Error())
}

func atoiForm(r *http.Request, key string) int {
	value, _ := strconv.Atoi(r.FormValue(key))
	return value
}

func atoi64Form(r *http.Request, key string) int64 {
	value, _ := strconv.ParseInt(r.FormValue(key), 10, 64)
	return value
}
