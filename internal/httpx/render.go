package httpx

import (
	"bytes"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
)

// RenderHTML renders comp to w with Content-Type "text/html; charset=utf-8". If rendering fails it logs the error with the request path and responds with HTTP 500.
func RenderHTML(w http.ResponseWriter, r *http.Request, logger *slog.Logger, comp templ.Component) {
	var buf bytes.Buffer
	if err := comp.Render(r.Context(), &buf); err != nil {
		logger.Error("render", "err", err, "path", r.URL.Path)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = buf.WriteTo(w)
}

// Render404 writes the 404 page with HTTP 404, falling back to a 500 plain-text error if template rendering fails.
func Render404(w http.ResponseWriter, r *http.Request, logger *slog.Logger, layout Layout) {
	RenderComponent404(w, r, logger, Page404(layout))
}

func RenderComponent404(w http.ResponseWriter, r *http.Request, logger *slog.Logger, comp templ.Component) {
	var buf bytes.Buffer
	if err := comp.Render(r.Context(), &buf); err != nil {
		logger.Error("render", "err", err, "path", r.URL.Path)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	_, _ = buf.WriteTo(w)
}
