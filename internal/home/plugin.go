package home

import (
	"net/http"

	authapp "github.com/hayakawakaki/go-racp/internal/auth/app"
	authinfra "github.com/hayakawakaki/go-racp/internal/auth/infra"
	authtransport "github.com/hayakawakaki/go-racp/internal/auth/transport"
	"github.com/hayakawakaki/go-racp/internal/home/transport"
	platinfra "github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/plugin"
)

// init registers the home plugin.
func init() {
	plugin.Register(plugin.Plugin{Name: "home", Mount: mount})
}

// mount the home plugin.
func mount(mux *http.ServeMux, in *platinfra.Infra) {
	userRepo := authinfra.NewRepository(in.MainDB)
	sessRepo := authinfra.NewSessionRepository(in.MainDB)

	userSvc := authapp.NewService(userRepo)
	sessSvc := authapp.NewSessionService(sessRepo)

	secure := in.Config.Env.Mode != "development"
	authH := authtransport.NewHandler(userSvc, sessSvc, in.Logger, secure)

	homeH := transport.NewHandler(userSvc, in.Logger)
	homeH.RegisterRoutes(mux, authH.WithSession)
}
