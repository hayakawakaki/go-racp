package home

import (
	"net/http"

	accountapp "github.com/hayakawakaki/go-racp/internal/account/app"
	accountinfra "github.com/hayakawakaki/go-racp/internal/account/infra"
	accounttransport "github.com/hayakawakaki/go-racp/internal/account/transport"
	"github.com/hayakawakaki/go-racp/internal/home/transport"
	platinfra "github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/plugin"
)

// init registers the home plugin.
func init() {
	plugin.Register(plugin.Plugin{Name: "home", Mount: mount})
}

// mount registers the home HTTP routes on mux using the provided Infra by creating authentication repositories and services, configuring session middleware (secure when Env.Mode != "development"), and attaching the handler.
func mount(mux *http.ServeMux, in *platinfra.Infra) {
	userRepo := accountinfra.NewRepository(in.MainDB)
	sessRepo := accountinfra.NewSessionRepository(in.MainDB)

	userSvc := accountapp.NewService(userRepo)
	sessSvc := accountapp.NewSessionService(sessRepo, in.Config.App.TTL.Session)

	secure := in.Config.Env.Mode != "development"
	withSession := accounttransport.WithSession(sessSvc, in.Logger, secure)

	homeH := transport.NewHandler(userSvc, transport.HandlerConfig{
		Logger:  in.Logger,
		General: in.Config.App.General,
	})
	homeH.RegisterRoutes(mux, withSession)
}
