package transport

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hayakawakaki/go-racp/internal/httpx"
	newsapp "github.com/hayakawakaki/go-racp/internal/news/app"
	"github.com/hayakawakaki/go-racp/internal/news/domain"
)

const maxHTMLFormBytes = 128 << 10

func (h *Handler) htmlList(w http.ResponseWriter, r *http.Request) {
	categoryKey := r.URL.Query().Get("category")
	items, err := h.fetchList(r, categoryKey)
	if err != nil {
		h.logger.Error("news: list", "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if categoryKey != "" && !h.svc.Categories().Has(categoryKey) {
		categoryKey = ""
	}

	state := NewsListState{
		Items:            items,
		Categories:       h.svc.Categories().All(),
		SelectedCategory: categoryKey,
	}
	httpx.RenderHTML(w, r, h.logger, newsListPage(h.layout(), state))
}

func (h *Handler) htmlDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		httpx.Render404(w, r, h.logger, h.layout())
		return
	}
	item, err := h.svc.GetByID(r.Context(), id)
	if errors.Is(err, domain.ErrNotFound) {
		httpx.Render404(w, r, h.logger, h.layout())
		return
	}
	if err != nil {
		h.logger.Error("news: detail", "err", err, "id", id)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	state := NewsDetailState{
		Item:     item,
		BodyHTML: h.renderer.Render(item.Body),
	}
	httpx.RenderHTML(w, r, h.logger, newsDetailPage(h.layout(), state))
}

func (h *Handler) htmlCreate(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxHTMLFormBytes)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "request too large or malformed", http.StatusBadRequest)
		return
	}
	title := r.FormValue("title")
	body := r.FormValue("body")
	category := r.FormValue("category")

	id, err := h.svc.Create(r.Context(), title, body, category)
	if status, msg, handled := h.classifyMutationErr(err); handled {
		http.Error(w, msg, status)
		return
	}

	w.Header().Set("Location", "/news/"+strconv.FormatInt(id, 10))
	w.WriteHeader(http.StatusSeeOther)
}

func (h *Handler) htmlUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		httpx.Render404(w, r, h.logger, h.layout())
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxHTMLFormBytes)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "request too large or malformed", http.StatusBadRequest)
		return
	}
	title := r.FormValue("title")
	body := r.FormValue("body")
	category := r.FormValue("category")

	err := h.svc.Update(r.Context(), id, title, body, category)
	if errors.Is(err, domain.ErrNotFound) {
		httpx.Render404(w, r, h.logger, h.layout())
		return
	}
	if status, msg, handled := h.classifyMutationErr(err); handled {
		http.Error(w, msg, status)
		return
	}

	w.Header().Set("Location", "/news/"+strconv.FormatInt(id, 10))
	w.WriteHeader(http.StatusSeeOther)
}

func (h *Handler) htmlDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		httpx.Render404(w, r, h.logger, h.layout())
		return
	}
	err := h.svc.Delete(r.Context(), id)
	if errors.Is(err, domain.ErrNotFound) {
		httpx.Render404(w, r, h.logger, h.layout())
		return
	}
	if err != nil {
		h.logger.Error("news: delete", "err", err, "id", id)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Location", "/news")
	w.WriteHeader(http.StatusSeeOther)
}

func (h *Handler) classifyMutationErr(err error) (status int, msg string, handled bool) {
	switch {
	case err == nil:
		return 0, "", false
	case errors.Is(err, domain.ErrTitleEmpty),
		errors.Is(err, domain.ErrTitleTooLong),
		errors.Is(err, domain.ErrBodyEmpty),
		errors.Is(err, domain.ErrBodyTooLong),
		errors.Is(err, domain.ErrInvalidCategory):
		return http.StatusBadRequest, err.Error(), true
	default:
		h.logger.Error("news: mutation", "err", err)
		return http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), true
	}
}

func (h *Handler) fetchList(r *http.Request, category string) ([]newsapp.NewsItem, error) {
	if category != "" && h.svc.Categories().Has(category) {
		items, err := h.svc.ListByCategory(r.Context(), category)
		if err != nil {
			return nil, fmt.Errorf("transport.Handler.fetchList: %w", err)
		}

		return items, nil
	}

	items, err := h.svc.List(r.Context())
	if err != nil {
		return nil, fmt.Errorf("transport.Handler.fetchList: %w", err)
	}

	return items, nil
}

func parseID(r *http.Request) (int64, bool) {
	raw := r.PathValue("id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}

	return id, true
}
