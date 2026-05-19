package transport

import (
	"errors"
	"net/http"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/news/domain"
	"github.com/hayakawakaki/go-racp/internal/httpx"
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
	category := r.URL.Query().Get(fieldCategory)
	if category != "" && !h.svc.Categories().Has(category) {
		_ = httpx.WriteJSON(w, http.StatusBadRequest, map[string]any{
			jsonErrorKey: "unknown category",
			"valid":      validCategoryKeys(h.svc.Categories()),
		})
		return
	}

	items, err := h.fetchList(r, category)
	if err != nil {
		h.logger.Error("news: api list", "err", err)
		_ = httpx.WriteJSON(w, http.StatusInternalServerError, map[string]string{jsonErrorKey: "internal error"})
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
	_ = httpx.WriteJSON(w, http.StatusOK, out)
}

func validCategoryKeys(resolver domain.CategoryResolver) []string {
	cats := resolver.All()
	out := make([]string, len(cats))
	for i, c := range cats {
		out[i] = c.Key
	}

	return out
}

func (h *Handler) jsonGet(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		_ = httpx.WriteJSON(w, http.StatusNotFound, map[string]string{jsonErrorKey: "news not found"})
		return
	}
	item, err := h.svc.GetByID(r.Context(), id)
	if errors.Is(err, domain.ErrNotFound) {
		_ = httpx.WriteJSON(w, http.StatusNotFound, map[string]string{jsonErrorKey: "news not found"})
		return
	}
	if err != nil {
		h.logger.Error("news: api get", "err", err, "id", id)
		_ = httpx.WriteJSON(w, http.StatusInternalServerError, map[string]string{jsonErrorKey: "internal error"})
		return
	}

	_ = httpx.WriteJSON(w, http.StatusOK, newsJSON{
		ID:        item.ID,
		Title:     item.Title,
		Body:      item.Body,
		Category:  item.Category,
		CreatedAt: item.CreatedAt.UTC(),
	})
}
