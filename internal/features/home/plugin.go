package home

import (
	"net/http"

	accountapp "github.com/hayakawakaki/go-racp/internal/features/account/app"
	accountinfra "github.com/hayakawakaki/go-racp/internal/features/account/infra"
	"github.com/hayakawakaki/go-racp/internal/features/home/transport"
	platinfra "github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/plugin"
	"github.com/hayakawakaki/go-racp/internal/routes"
)

// init registers the home plugin.
func init() {
	plugin.Register(plugin.Plugin{Name: "home", Mount: mount})
}

// mount registers the home HTTP routes on mux using the provided Infra by creating authentication repositories and services, configuring session middleware (secure when Env.Mode != "development"), and attaching the handler.
func mount(reg *routes.Registry, mux *http.ServeMux, in *platinfra.Infra) {
	userRepo := accountinfra.NewRepository(in.MainDB)
	userSvc := accountapp.NewService(userRepo)

	homeH := transport.NewHandler(userSvc, transport.HandlerConfig{
		Logger:  in.Logger,
		General: in.Config.App.General,
	})
	homeH.RegisterRoutes(reg, mux)
}
