package auth

import (
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/auth/app"
	"github.com/hayakawakaki/go-racp/internal/auth/infra"
	"github.com/hayakawakaki/go-racp/internal/auth/transport"
	platinfra "github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/plugin"
)

// init registers the "auth" plugin with the plugin framework so its mount
// callback will be invoked to attach authentication routes during setup.
func init() {
	plugin.Register(plugin.Plugin{Name: "auth", Mount: mount})
}

// mount registers the auth HTTP routes on mux by constructing the repository,
// service, and HTTP handler from the provided infrastructure.
func mount(mux *http.ServeMux, in *platinfra.Infra) {
	userRepo := infra.NewRepository(in.MainDB)
	authSvc := app.NewService(userRepo)

	h := transport.NewHandler(authSvc, in.Logger)
	h.RegisterRoutes(mux)
}
