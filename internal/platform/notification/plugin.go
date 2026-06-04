package notification

import (
	"net/http"

	coreinfra "github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
	"github.com/hayakawakaki/go-racp/internal/platform/notification/transport"
	"github.com/hayakawakaki/go-racp/internal/platform/plugin"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
)

func init() {
	plugin.Register(plugin.Plugin{Name: "notification", Mount: mount})
}

func mount(reg *routes.Registry, mux *http.ServeMux, in *coreinfra.Infra) {
	layout := httpx.Layout{GeneralConfig: in.Config.App.General}
	handler := transport.NewHandler(in.Notifications, in.Logger, layout)
	handler.RegisterRoutes(reg, mux)
}
