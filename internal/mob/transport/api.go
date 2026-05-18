package transport

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/hayakawakaki/go-racp/internal/httpx"
	mobapp "github.com/hayakawakaki/go-racp/internal/mob/app"
	"github.com/hayakawakaki/go-racp/internal/mob/domain"
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

	_ = httpx.WriteJSON(w, http.StatusOK, mobapp.ToDTO(mob))
}
