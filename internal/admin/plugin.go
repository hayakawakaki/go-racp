package admin

import (
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/admin/transport"
	platinfra "github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/item"
	"github.com/hayakawakaki/go-racp/internal/mob"
	"github.com/hayakawakaki/go-racp/internal/plugin"
	"github.com/hayakawakaki/go-racp/internal/routes"
)

func init() {
	plugin.Register(plugin.Plugin{
		Name:  "admin",
		Mount: mount,
	})
}

func mount(reg *routes.Registry, mux *http.ServeMux, in *platinfra.Infra) {
	h := transport.NewHandler(transport.HandlerConfig{
		Logger:     in.Logger,
		General:    in.Config.App.General,
		ItemStatus: item.BuildService(in),
		MobStatus:  mob.BuildService(in),
	})
	h.RegisterRoutes(reg, mux)
}
