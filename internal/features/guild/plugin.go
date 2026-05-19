package guild

import (
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/features/guild/app"
	"github.com/hayakawakaki/go-racp/internal/features/guild/infra"
	"github.com/hayakawakaki/go-racp/internal/features/guild/transport"
	coreinfra "github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/platform/plugin"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
)

func init() {
	plugin.Register(plugin.Plugin{Name: "guild", Mount: mount})
}

func mount(reg *routes.Registry, mux *http.ServeMux, in *coreinfra.Infra) {
	svc := BuildService(in)
	handler := transport.NewHandler(svc, transport.HandlerConfig{
		Logger:  in.Logger,
		General: in.Config.App.General,
	})
	handler.RegisterRoutes(reg, mux)
}

func BuildService(in *coreinfra.Infra) *app.Service {
	repo := infra.NewRepository(in.MainDB)

	return app.NewService(repo)
}
