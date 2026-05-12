package httpx

import (
	"bytes"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
)

// RenderHTML renders comp to w with Content-Type "text/html; charset=utf-8"; if rendering fails it logs the error with the request path and responds with HTTP 500.
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

func Render404(w http.ResponseWriter, r *http.Request, logger *slog.Logger, layout Layout) {
	var buf bytes.Buffer
	if err := Page404(layout).Render(r.Context(), &buf); err != nil {
		logger.Error("render", "err", err, "path", r.URL.Path)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	_, _ = buf.WriteTo(w)
}
