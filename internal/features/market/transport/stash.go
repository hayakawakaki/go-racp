package transport

import (
	"log/slog"
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/features/account/transport/middleware"
	"github.com/hayakawakaki/go-racp/internal/features/market/app"
	"github.com/hayakawakaki/go-racp/internal/features/market/domain"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
)

type stashItemResponse struct {
	Card       [4]int `json:"card"`
	OptionID   [5]int `json:"option_id"`
	OptionVal  [5]int `json:"option_val"`
	OptionParm [5]int `json:"option_parm"`
	ID         int64  `json:"id"`
	UniqueID   int64  `json:"unique_id"`
	ExpireTime int64  `json:"expire_time"`
	AccountID  int    `json:"account_id"`
	NameID     int    `json:"name_id"`
	Amount     int    `json:"amount"`
	Equip      int    `json:"equip"`
	Identify   int    `json:"identify"`
	Refine     int    `json:"refine"`
	Attribute  int    `json:"attribute"`
	Bound      int    `json:"bound"`
	Grade      int    `json:"grade"`
}

type stashResponse struct {
	Items      []stashItemResponse `json:"items"`
	SlotsUsed  int                 `json:"slots_used"`
	SlotsTotal int                 `json:"slots_total"`
	Locked     bool                `json:"locked"`
}

type Handler struct {
	stash  *app.StashService
	logger *slog.Logger
}

func NewHandler(stash *app.StashService, logger *slog.Logger) *Handler {
	return &Handler{stash: stash, logger: logger}
}

func (h *Handler) RegisterRoutes(reg *routes.Registry, mux *http.ServeMux) {
	reg.Wrap(mux, "Market.View", "GET /market/stash", http.HandlerFunc(h.stashJSON))
}

func (h *Handler) stashJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		_ = httpx.WriteJSONError(w, http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
		return
	}

	view, err := h.stash.View(r.Context(), session.UserID)
	if err != nil {
		h.logger.Error("market: stash view", "err", err)
		_ = httpx.WriteJSONError(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}

	response := toStashResponse(view)
	if err := httpx.WriteJSON(w, http.StatusOK, response); err != nil {
		h.logger.Error("market: write stash", "err", err)
	}
}

func toStashResponse(view app.StashView) stashResponse {
	items := make([]stashItemResponse, len(view.Items))
	for index, item := range view.Items {
		items[index] = toStashItemResponse(item)
	}

	return stashResponse{
		Items:      items,
		SlotsUsed:  view.SlotsUsed,
		SlotsTotal: view.SlotsTotal,
		Locked:     view.Locked,
	}
}

func toStashItemResponse(item domain.StashItem) stashItemResponse {
	return stashItemResponse{
		Card:       item.Card,
		OptionID:   item.OptionID,
		OptionVal:  item.OptionVal,
		OptionParm: item.OptionParm,
		ID:         item.ID,
		UniqueID:   item.UniqueID,
		ExpireTime: item.ExpireTime,
		AccountID:  item.AccountID,
		NameID:     item.NameID,
		Amount:     item.Amount,
		Equip:      item.Equip,
		Identify:   item.Identify,
		Refine:     item.Refine,
		Attribute:  item.Attribute,
		Bound:      item.Bound,
		Grade:      item.Grade,
	}
}
