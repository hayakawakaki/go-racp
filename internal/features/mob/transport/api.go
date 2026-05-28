package transport

import (
	"errors"
	"net/http"
	"strconv"

	itemdomain "github.com/hayakawakaki/go-racp/internal/features/item/domain"
	"github.com/hayakawakaki/go-racp/internal/features/mob/app"
	"github.com/hayakawakaki/go-racp/internal/features/mob/domain"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
	"github.com/hayakawakaki/go-racp/server/config"
)

type apiError struct {
	Error string `json:"error"`
	ID    int    `json:"id"`
}

func (h *Handler) apiDetail(w http.ResponseWriter, r *http.Request) {
	idText := r.PathValue("id")
	id, err := strconv.Atoi(idText)
	if err != nil || id < 1 {
		_ = httpx.WriteJSON(w, http.StatusNotFound, apiError{Error: "mob not found", ID: 0})
		return
	}

	mob, err := h.svc.Get(r.Context(), id)
	if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrEmptySnapshot) {
		_ = httpx.WriteJSON(w, http.StatusNotFound, apiError{Error: "mob not found", ID: id})
		return
	}
	if err != nil {
		h.logger.Error("mob: api detail", "err", err, "id", id)
		_ = httpx.WriteJSON(w, http.StatusInternalServerError, apiError{Error: "internal error", ID: id})
		return
	}

	dto := app.ToDTO(mob)
	applyRates(&dto, h.currentItemLookup(), h.general.Rates)
	_ = httpx.WriteJSON(w, http.StatusOK, dto)
}

func applyRates(dto *app.MobDTO, lookup ItemLookup, rates config.RatesConfig) {
	dto.BaseExp = scaleRate(dto.BaseExp, rates.ExpRate)
	dto.JobExp = scaleRate(dto.JobExp, rates.JobRate)
	dto.MvpExp = scaleRate(dto.MvpExp, rates.ExpRate)
	scaleDropRates(dto.Drops, lookup, false, rates)
	scaleDropRates(dto.MvpDrops, lookup, true, rates)
}

func scaleDropRates(drops []app.DropDTO, lookup ItemLookup, isMVP bool, rates config.RatesConfig) {
	for index := range drops {
		itemType := itemdomain.ItemTypeUnknown
		if lookup != nil {
			if item := lookup.LookupByAegis(drops[index].ItemAegis); item != nil {
				itemType = item.Type
			}
		}
		drops[index].Rate = min(scaleRate(drops[index].Rate, categoryRate(itemType, isMVP, rates)), 10000)
	}
}
