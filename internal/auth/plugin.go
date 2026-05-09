package auth

import (
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/auth/app"
	"github.com/hayakawakaki/go-racp/internal/auth/infra"
	"github.com/hayakawakaki/go-racp/internal/auth/transport"
	platinfra "github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/plugin"
)

// init registers the "auth" plugin with the plugin registry by providing its Mount function.
func init() {
	plugin.Register(plugin.Plugin{Name: "auth", Mount: mount})
}

// mount registers authentication HTTP routes on mux.
// It wires user and session repositories and services, creates the transport handler (marked secure unless Env.Mode == "development"), and registers its routes.
func mount(mux *http.ServeMux, in *platinfra.Infra) {
	userRepo := infra.NewRepository(in.MainDB)
	sessRepo := infra.NewSessionRepository(in.MainDB)

	authSvc := app.NewService(userRepo)
	sessSvc := app.NewSessionService(sessRepo)

	secure := in.Config.Env.Mode != "development"

	h := transport.NewHandler(authSvc, sessSvc, in.Logger, secure)
	h.RegisterRoutes(mux)
}
