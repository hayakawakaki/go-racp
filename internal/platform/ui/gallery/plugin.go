package gallery

import (
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
	"github.com/hayakawakaki/go-racp/internal/platform/plugin"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
	"github.com/hayakawakaki/go-racp/internal/platform/ui"
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
		httpx.RenderHTML(w, r, in.Logger, ui.GalleryIndex(ui.List()))
	})

	mux.HandleFunc("GET /_dev/components/{name}", func(w http.ResponseWriter, r *http.Request) {
		c, ok := ui.Get(r.PathValue("name"))
		if !ok {
			http.NotFound(w, r)
			return
		}

		httpx.RenderHTML(w, r, in.Logger, ui.GalleryDetail(c))
	})
}
