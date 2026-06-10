package market

import (
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/features/market/app"
	"github.com/hayakawakaki/go-racp/internal/features/market/infra"
	"github.com/hayakawakaki/go-racp/internal/features/market/transport"
	coreinfra "github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/platform/plugin"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
)

func init() {
	plugin.Register(plugin.Plugin{Name: "market", Mount: mount})
}

func mount(reg *routes.Registry, mux *http.ServeMux, in *coreinfra.Infra) {
	stashRepo := infra.NewStashRepository(in.MainDB)
	stashService := app.NewStashService(stashRepo, 0)

	handler := transport.NewHandler(stashService, in.Logger)
	handler.RegisterRoutes(reg, mux)
}
