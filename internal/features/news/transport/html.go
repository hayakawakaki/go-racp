package transport

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hayakawakaki/go-racp/internal/features/news/app"
	"github.com/hayakawakaki/go-racp/internal/features/news/domain"
	"github.com/hayakawakaki/go-racp/internal/features/news/transport/state"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
)

const (
	maxHTMLFormBytes = 128 << 10
	fieldTitle       = "title"
	fieldBody        = "body"
	fieldCategory    = "category"
	categoryAll      = "All"
)

func (h *Handler) htmlList(w http.ResponseWriter, r *http.Request) {
	categoryKey := r.URL.Query().Get(fieldCategory)
	if categoryKey == categoryAll || !h.svc.Categories().Has(categoryKey) {
		categoryKey = ""
	}
	items, err := h.fetchList(r, categoryKey)
	if err != nil {
		h.logger.Error("news: list", "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	selected := categoryKey
	if selected == "" {
		selected = categoryAll
	}
	s := state.NewsListState{
		Items:            items,
		Categories:       h.svc.Categories().All(),
		SelectedCategory: selected,
		CanManage:        h.canManage(r),
	}
	httpx.RenderHTML(w, r, h.logger, h.theme.NewsListPage(h.layout(), s))
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

	s := state.NewsDetailState{
		Item:      item,
		BodyHTML:  h.renderer.Render(item.Body),
		CanManage: h.canManage(r),
	}
	httpx.RenderHTML(w, r, h.logger, h.theme.NewsDetailPage(h.layout(), s))
}

func (h *Handler) htmlCreateForm(w http.ResponseWriter, r *http.Request) {
	s := h.newCreateFormState("", "", "")
	httpx.RenderHTML(w, r, h.logger, h.theme.NewsFormPage(h.layout(), s))
}

func (h *Handler) htmlPreview(w http.ResponseWriter, r *http.Request) {
	if err := httpx.ParseForm(w, r, maxHTMLFormBytes); err != nil {
		http.Error(w, "request too large or malformed", http.StatusBadRequest)
		return
	}
	rendered := h.renderer.Render(r.PostFormValue(fieldBody))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := w.Write([]byte(rendered)); err != nil { //nolint:gosec // rendered is already sanitized via goldmark + bluemonday in the renderer.
		h.logger.Error("news: preview write", "err", err)
	}
}

func (h *Handler) htmlCreate(w http.ResponseWriter, r *http.Request) {
	if err := httpx.ParseForm(w, r, maxHTMLFormBytes); err != nil {
		http.Error(w, "request too large or malformed", http.StatusBadRequest)
		return
	}
	title := r.PostFormValue(fieldTitle)
	body := r.PostFormValue(fieldBody)
	category := r.PostFormValue(fieldCategory)

	id, err := h.svc.Create(r.Context(), title, body, category)
	if err == nil {
		w.Header().Set("Location", "/news/"+strconv.FormatInt(id, 10))
		w.WriteHeader(http.StatusSeeOther)
		return
	}
	if field, msg := fieldFromErr(err); field != "" {
		s := h.newCreateFormState(title, body, category)
		s.Errors = map[string]string{field: msg}
		httpx.RenderHTML(w, r, h.logger, h.theme.NewsFormPage(h.layout(), s))
		return
	}
	h.logger.Error("news: create", "err", err)
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

func (h *Handler) htmlEditForm(w http.ResponseWriter, r *http.Request) {
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
		h.logger.Error("news: edit form", "err", err, "id", id)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	s := h.newEditFormState(id, item.Title, item.Body, item.Category)
	httpx.RenderHTML(w, r, h.logger, h.theme.NewsFormPage(h.layout(), s))
}

func (h *Handler) htmlUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		httpx.Render404(w, r, h.logger, h.layout())
		return
	}
	if err := httpx.ParseForm(w, r, maxHTMLFormBytes); err != nil {
		http.Error(w, "request too large or malformed", http.StatusBadRequest)
		return
	}
	title := r.PostFormValue(fieldTitle)
	body := r.PostFormValue(fieldBody)
	category := r.PostFormValue(fieldCategory)

	err := h.svc.Update(r.Context(), id, title, body, category)
	if err == nil {
		w.Header().Set("Location", "/news/"+strconv.FormatInt(id, 10))
		w.WriteHeader(http.StatusSeeOther)
		return
	}
	if errors.Is(err, domain.ErrNotFound) {
		httpx.Render404(w, r, h.logger, h.layout())
		return
	}
	if field, msg := fieldFromErr(err); field != "" {
		s := h.newEditFormState(id, title, body, category)
		s.Errors = map[string]string{field: msg}
		httpx.RenderHTML(w, r, h.logger, h.theme.NewsFormPage(h.layout(), s))
		return
	}
	h.logger.Error("news: update", "err", err, "id", id)
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
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

func (h *Handler) newCreateFormState(title, body, category string) state.NewsFormState {
	categories := h.svc.Categories().All()
	if category == "" && len(categories) > 0 {
		category = categories[0].Key
	}

	return state.NewsFormState{
		Action:         "/news",
		PageTitle:      "New news post",
		Submit:         "Create",
		Title:          title,
		Body:           body,
		Category:       category,
		Categories:     categories,
		InitialPreview: h.renderer.Render(body),
	}
}

func (h *Handler) newEditFormState(id int64, title, body, category string) state.NewsFormState {
	return state.NewsFormState{
		Action:         fmt.Sprintf("/news/%d/edit", id),
		PageTitle:      "Edit: " + title,
		Submit:         "Save",
		Title:          title,
		Body:           body,
		Category:       category,
		Categories:     h.svc.Categories().All(),
		InitialPreview: h.renderer.Render(body),
	}
}

func fieldFromErr(err error) (field, message string) {
	switch {
	case errors.Is(err, domain.ErrTitleEmpty):
		return fieldTitle, "Title is required"
	case errors.Is(err, domain.ErrTitleTooLong):
		return fieldTitle, "Title is too long"
	case errors.Is(err, domain.ErrBodyEmpty):
		return fieldBody, "Body is required"
	case errors.Is(err, domain.ErrBodyTooLong):
		return fieldBody, "Body is too long"
	case errors.Is(err, domain.ErrInvalidCategory):
		return fieldCategory, "Invalid category"
	}

	return "", ""
}

func (h *Handler) fetchList(r *http.Request, category string) ([]app.NewsItem, error) {
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
