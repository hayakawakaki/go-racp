package httpx

import (
	"bytes"
	"cmp"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
)

// RenderHTML renders comp to w with Content-Type "text/html; charset=utf-8". If rendering fails it logs the error with the request path and responds with HTTP 500.
func RenderHTML(w http.ResponseWriter, r *http.Request, logger *slog.Logger, comp templ.Component) {
	var buf bytes.Buffer
	if err := comp.Render(r.Context(), &buf); err != nil {
		cmp.Or(logger, slog.Default()).Error("render", "err", err, "path", r.URL.Path)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = buf.WriteTo(w)
}

// Render404 writes the 404 page with HTTP 404, falling back to a 500 plain-text error if template rendering fails.
func Render404(w http.ResponseWriter, r *http.Request, logger *slog.Logger, layout Layout) {
	if ActivePage404 == nil {
		panic("httpx: ActivePage404 is nil; blank-import \"github.com/hayakawakaki/go-racp/themes/default/platform/httpx\" or your active theme's platform/httpx package")
	}

	RenderComponent404(w, r, logger, ActivePage404(layout))
}

func RenderComponent404(w http.ResponseWriter, r *http.Request, logger *slog.Logger, comp templ.Component) {
	var buf bytes.Buffer
	if err := comp.Render(r.Context(), &buf); err != nil {
		cmp.Or(logger, slog.Default()).Error("render", "err", err, "path", r.URL.Path)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	_, _ = buf.WriteTo(w)
}

// Render429 writes the 429 page with HTTP 429, falling back to a 500 plain-text error if template rendering fails.
func Render429(w http.ResponseWriter, r *http.Request, logger *slog.Logger, layout Layout) {
	if ActivePage429 == nil {
		panic("httpx: ActivePage429 is nil; blank-import \"github.com/hayakawakaki/go-racp/themes/default/platform/httpx\" or your active theme's platform/httpx package")
	}

	var buf bytes.Buffer
	if err := ActivePage429(layout).Render(r.Context(), &buf); err != nil {
		cmp.Or(logger, slog.Default()).Error("render", "err", err, "path", r.URL.Path)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusTooManyRequests)
	_, _ = buf.WriteTo(w)
}
