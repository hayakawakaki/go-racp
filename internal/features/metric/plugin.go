package metric

import (
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/features/metric/transport"
	coreinfra "github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/platform/plugin"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
)

func init() {
	plugin.Register(plugin.Plugin{Name: "metric", Mount: mount})
}

func mount(reg *routes.Registry, mux *http.ServeMux, in *coreinfra.Infra) {
	handler := transport.NewHandler(in.Metric, in.Logger)
	handler.RegisterRoutes(reg, mux)
}
