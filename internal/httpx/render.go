package httpx

import (
	"bytes"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
)

// RenderHTML writes the HTML output produced by comp to w.
//
// If comp.Render returns an error, RenderHTML logs the error (including the request path)
// and responds with HTTP 500 using http.StatusText(http.StatusInternalServerError).
// On success it sets the Content-Type header to "text/html; charset=utf-8" and writes
// the rendered bytes to the response.
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
