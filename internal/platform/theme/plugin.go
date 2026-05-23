package theme

import (
	"io/fs"
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
	"github.com/hayakawakaki/go-racp/internal/platform/plugin"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
	themesdefault "github.com/hayakawakaki/go-racp/themes/default"
)

func init() {
	plugin.Register(plugin.Plugin{
		Name:  "theme",
		Mount: mount,
	})
}

func mount(_ *routes.Registry, mux *http.ServeMux, in *infra.Infra) {
	activeTheme := in.Config.App.General.Theme
	urlPrefix := "/themes/" + activeTheme + "/static/"

	var handler http.Handler

	if in.Config.Env.Mode == "development" {
		handler = http.FileServer(http.Dir("themes/" + activeTheme + "/static"))
	} else {
		sub, err := fs.Sub(themesdefault.Static, "static")
		if err != nil {
			panic("theme: fs.Sub themes/default/static: " + err.Error())
		}

		handler = http.FileServer(http.FS(sub))
	}

	mux.Handle(urlPrefix, http.StripPrefix(urlPrefix, handler))

	layout := httpx.Layout{GeneralConfig: in.Config.App.General}
	themesdefault.MountRoutes(mux, layout)

	in.Logger.Info("theme assets mounted", "prefix", urlPrefix, "mode", in.Config.Env.Mode)
}
