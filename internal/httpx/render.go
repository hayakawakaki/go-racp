package httpx

import (
	"bytes"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
)

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
