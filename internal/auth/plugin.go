package auth

import (
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/auth/app"
	"github.com/hayakawakaki/go-racp/internal/auth/infra"
	"github.com/hayakawakaki/go-racp/internal/auth/transport"
	platinfra "github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/plugin"
)

// init registers the auth plugin
func init() {
	plugin.Register(plugin.Plugin{Name: "auth", Mount: mount})
}

// mount the auth plugin
func mount(mux *http.ServeMux, in *platinfra.Infra) {
	userRepo := infra.NewRepository(in.MainDB)
	authSvc := app.NewService(userRepo)

	h := transport.NewHandler(authSvc, in.Logger)
	h.RegisterRoutes(mux)
}
