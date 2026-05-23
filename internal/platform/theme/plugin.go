package theme

import (
	"fmt"
	"io/fs"
	"net/http"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
	"github.com/hayakawakaki/go-racp/internal/platform/plugin"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
	"github.com/hayakawakaki/go-racp/internal/platform/themepage"
	themesdefault "github.com/hayakawakaki/go-racp/themes/default"
)

func init() {
	plugin.Register(plugin.Plugin{
		Name:  "theme",
		Mount: mount,
	})
}

func mount(reg *routes.Registry, mux *http.ServeMux, in *infra.Infra) {
	activeTheme := in.Config.App.General.Theme
	devMode := in.Config.Env.Mode == "development"

	themepage.DevMode = devMode
	themepage.DiskRoot = filepath.Join("themes", "default")

	urlPrefix := "/themes/" + activeTheme + "/static/"

	var handler http.Handler

	if devMode {
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
	themesdefault.MountRoutes(reg, mux, layout)

	if devMode {
		mux.HandleFunc("GET /_dev/routes", devRoutesHandler(reg))
	}

	in.Logger.Info("theme",
		"name", themesdefault.ThemeName,
		"version", themesdefault.ThemeVersion,
		"pages", themesdefault.PageCount,
		"prefix", urlPrefix,
		"mode", in.Config.Env.Mode,
	)
}

func devRoutesHandler(reg *routes.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		snapshot := reg.RoutesSnapshot()

		slices.SortFunc(snapshot, func(a, b routes.RouteInfo) int {
			return strings.Compare(a.Tag, b.Tag)
		})

		var themePages, gated []routes.RouteInfo

		for _, ri := range snapshot {
			if strings.HasPrefix(ri.Tag, "ThemePages.") {
				themePages = append(themePages, ri)
			} else {
				gated = append(gated, ri)
			}
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")

		fmt.Fprintln(w, "Theme pages:")

		for _, ri := range themePages {
			fmt.Fprintf(w, "  %-32s %s\n", ri.Tag, ri.Pattern)
		}

		fmt.Fprintln(w)
		fmt.Fprintln(w, "Gated routes:")

		for _, ri := range gated {
			fmt.Fprintf(w, "  %-32s %s\n", ri.Tag, ri.Pattern)
		}
	}
}
