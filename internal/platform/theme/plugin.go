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
	"github.com/hayakawakaki/go-racp/internal/platform/themecfg"
	"github.com/hayakawakaki/go-racp/internal/platform/themepage"
)

func init() {
	plugin.Register(plugin.Plugin{
		Name:  "theme",
		Mount: mount,
	})
}

func mount(reg *routes.Registry, mux *http.ServeMux, in *infra.Infra) {
	if err := themecfg.LoadCfg(ActiveName); err != nil {
		panic(fmt.Errorf("theme: load config: %w", err))
	}

	devMode := in.Config.Env.Mode == "development"

	themepage.DevMode = devMode
	themepage.DiskRoot = filepath.Join("themes", ActiveName)

	mountStaticHandler(mux, devMode, ActiveName, ActiveStatic)
	if ActiveName != DefaultName {
		mountStaticHandler(mux, devMode, DefaultName, DefaultStatic)
	}

	layout := httpx.Layout{GeneralConfig: in.Config.App.General}
	ActiveMountRoutes(reg, mux, layout)

	if devMode {
		mux.HandleFunc("GET /_dev/routes", devRoutesHandler(reg))
	}

	in.Logger.Info("theme",
		"name", ActiveName,
		"version", ActiveVersion,
		"pages", ActivePageCount,
		"prefix", "/themes/"+ActiveName+"/static/",
		"mode", in.Config.Env.Mode,
	)
}

func mountStaticHandler(mux *http.ServeMux, devMode bool, themeName string, embedded fs.FS) {
	urlPrefix := "/themes/" + themeName + "/static/"

	var handler http.Handler

	var etags map[string]string
	if devMode {
		handler = http.FileServer(http.Dir("themes/" + themeName + "/static"))
	} else {
		sub, err := fs.Sub(embedded, "static")
		if err != nil {
			panic("theme: fs.Sub themes/" + themeName + "/static: " + err.Error())
		}

		handler = http.FileServer(http.FS(sub))

		tags, err := httpx.StaticETags(sub, urlPrefix)
		if err != nil {
			panic("theme: static etags " + themeName + ": " + err.Error())
		}

		etags = tags
	}

	mux.Handle(urlPrefix, httpx.StaticCache(http.StripPrefix(urlPrefix, handler), devMode, etags))
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
