package auth

import (
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/auth/app"
	"github.com/hayakawakaki/go-racp/internal/auth/infra"
	"github.com/hayakawakaki/go-racp/internal/auth/transport"
	platinfra "github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/plugin"
)

// init registers the "auth" plugin with the global plugin registry, using mount as its mount function.
func init() {
	plugin.Register(plugin.Plugin{Name: "auth", Mount: mount})
}

// mount wires the auth module into the provided ServeMux using resources from in.
// It constructs a user repository from in.MainDB, creates the authentication service
// and HTTP handler (using in.Logger), and registers the handler's routes on mux.
func mount(mux *http.ServeMux, in *platinfra.Infra) {
	userRepo := infra.NewRepository(in.MainDB)
	authSvc := app.NewService(userRepo)

	h := transport.NewHandler(authSvc, in.Logger)
	h.RegisterRoutes(mux)
}
