package transport

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/hayakawakaki/go-racp/internal/news/domain"
)

const jsonErrorKey = "error"

type newsJSON struct {
	CreatedAt time.Time `json:"created_at"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Category  string    `json:"category"`
	ID        int64     `json:"id"`
}

func (h *Handler) jsonList(w http.ResponseWriter, r *http.Request) {
	items, err := h.fetchList(r, r.URL.Query().Get("category"))
	if err != nil {
		h.logger.Error("news: api list", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{jsonErrorKey: "internal error"})
		return
	}

	out := make([]newsJSON, 0, len(items))
	for _, item := range items {
		out = append(out, newsJSON{
			ID:        item.ID,
			Title:     item.Title,
			Body:      item.Body,
			Category:  item.Category,
			CreatedAt: item.CreatedAt.UTC(),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) jsonGet(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{jsonErrorKey: "news not found"})
		return
	}
	item, err := h.svc.GetByID(r.Context(), id)
	if errors.Is(err, domain.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{jsonErrorKey: "news not found"})
		return
	}
	if err != nil {
		h.logger.Error("news: api get", "err", err, "id", id)
		writeJSON(w, http.StatusInternalServerError, map[string]string{jsonErrorKey: "internal error"})
		return
	}

	writeJSON(w, http.StatusOK, newsJSON{
		ID:        item.ID,
		Title:     item.Title,
		Body:      item.Body,
		Category:  item.Category,
		CreatedAt: item.CreatedAt.UTC(),
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
