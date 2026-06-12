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

	form := formReader{r: r}
	in := app.CreateInput{
		SellerAccountID: session.UserID,
		Kind:            form.intField("kind"),
		GiveStashItemID: form.int64Field("give_stash_item_id"),
		GiveUnitAmount:  form.intField("give_unit_amount"),
		GiveZeny:        form.int64Field("give_zeny"),
		GiveCashpoint:   form.intField("give_cashpoint"),
		WantNameID:      form.intField("want_nameid"),
		WantUnitAmount:  form.intField("want_unit_amount"),
		WantZeny:        form.int64Field("want_zeny"),
		WantCashpoint:   form.intField("want_cashpoint"),
		Quantity:        form.intField("quantity"),
	}
	if form.err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
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

	form := formReader{r: r}
	input := app.TakeInput{
		ListingID:        id,
		TakerAccountID:   session.UserID,
		Units:            form.intField("units"),
		TakerStashItemID: form.int64Field("taker_stash_item_id"),
	}
	if form.err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if takeErr := h.offers.Take(r.Context(), input); takeErr != nil {
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

var offerErrorStatuses = []struct {
	err    error
	status int
}{
	{domain.ErrListingNotFound, http.StatusNotFound},
	{domain.ErrStashItemNotFound, http.StatusNotFound},
	{domain.ErrStorageUnlocked, http.StatusConflict},
	{domain.ErrStorageFull, http.StatusConflict},
	{domain.ErrInsufficientFunds, http.StatusConflict},
	{domain.ErrListingInactive, http.StatusConflict},
	{domain.ErrInsufficientUnits, http.StatusConflict},
	{domain.ErrInvalidOffer, http.StatusBadRequest},
	{domain.ErrWantMismatch, http.StatusBadRequest},
	{domain.ErrItemBlacklisted, http.StatusBadRequest},
	{domain.ErrSelfTrade, http.StatusBadRequest},
}

func (h *Handler) writeOfferError(w http.ResponseWriter, r *http.Request, err error) {
	for _, mapping := range offerErrorStatuses {
		if errors.Is(err, mapping.err) {
			h.logger.Warn("market: offer rejected", "err", err, "path", r.URL.Path)
			_ = httpx.WriteJSONError(w, mapping.status, mapping.err.Error())

			return
		}
	}

	h.logger.Error("market: offer failed", "err", err, "path", r.URL.Path)
	_ = httpx.WriteJSONError(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
}

type formReader struct {
	err error
	r   *http.Request
}

func (f *formReader) intField(key string) int {
	if f.err != nil {
		return 0
	}

	raw := f.r.FormValue(key)
	if raw == "" {
		return 0
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		f.err = err

		return 0
	}

	return value
}

func (f *formReader) int64Field(key string) int64 {
	if f.err != nil {
		return 0
	}

	raw := f.r.FormValue(key)
	if raw == "" {
		return 0
	}

	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		f.err = err

		return 0
	}

	return value
}
