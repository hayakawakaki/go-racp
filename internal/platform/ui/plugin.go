package ui

import (
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
	"github.com/hayakawakaki/go-racp/internal/platform/plugin"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
)

func init() {
	plugin.Register(plugin.Plugin{
		Name:  "ui",
		Mount: mount,
	})
}

func mount(_ *routes.Registry, mux *http.ServeMux, in *infra.Infra) {
	if in.Config.Env.Mode != "development" {
		return
	}

	mux.HandleFunc("GET /_dev/components", func(w http.ResponseWriter, r *http.Request) {
		httpx.RenderHTML(w, r, in.Logger, galleryIndex(List()))
	})

	mux.HandleFunc("GET /_dev/components/{name}", func(w http.ResponseWriter, r *http.Request) {
		c, ok := Get(r.PathValue("name"))
		if !ok {
			http.NotFound(w, r)
			return
		}

		httpx.RenderHTML(w, r, in.Logger, galleryDetail(c))
	})
}
