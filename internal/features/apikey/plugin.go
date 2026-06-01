package apikey

import (
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/features/apikey/transport"
	coreinfra "github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/platform/plugin"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
	"github.com/hayakawakaki/go-racp/internal/platform/theme"
)

func init() {
	plugin.Register(plugin.Plugin{Name: "apikey", Mount: mount})
}

func mount(reg *routes.Registry, mux *http.ServeMux, in *coreinfra.Infra) {
	handler := transport.NewHandler(in.APIKeys, transport.HandlerConfig{
		Logger:  in.Logger,
		General: in.Config.App.General,
		Theme:   theme.Active,
	})
	handler.RegisterRoutes(reg, mux)
}
