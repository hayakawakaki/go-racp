// Package auth is the top-level auth plugin package. Its init function
// registers the "auth" plugin with the global plugin registry so that
// server.Start (via plugin.MountAll) can wire the auth routes into the HTTP
// mux without the server package importing auth directly.
package auth

import (
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/auth/app"
	"github.com/hayakawakaki/go-racp/internal/auth/infra"
	"github.com/hayakawakaki/go-racp/internal/auth/transport"
	platinfra "github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/plugin"
)

func init() {
	plugin.Register(plugin.Plugin{Name: "auth", Mount: mount})
}

// mount wires the auth dependency graph (repository → service → handler) and
// registers all auth HTTP routes on mux.
func mount(mux *http.ServeMux, in *platinfra.Infra) {
	userRepo := infra.NewRepository(in.MainDB)
	authSvc := app.NewService(userRepo)

	h := transport.NewHandler(authSvc, in.Logger)
	h.RegisterRoutes(mux)
}
