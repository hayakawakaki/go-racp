package account

import (
	"net/http"

	accountapp "github.com/hayakawakaki/go-racp/internal/account/app"
	"github.com/hayakawakaki/go-racp/internal/account/transport"
	authapp "github.com/hayakawakaki/go-racp/internal/auth/app"
	authinfra "github.com/hayakawakaki/go-racp/internal/auth/infra"
	authtransport "github.com/hayakawakaki/go-racp/internal/auth/transport"
	platinfra "github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/plugin"
)

func init() {
	plugin.Register(plugin.Plugin{
		Name:  "account",
		Mount: mount,
	})
}

func mount(mux *http.ServeMux, in *platinfra.Infra) {
	userRepo := authinfra.NewRepository(in.MainDB)
	sessRepo := authinfra.NewSessionRepository(in.MainDB)
	sessSvc := authapp.NewSessionService(sessRepo, in.Config.App.TTL.Session)

	svc := accountapp.NewService(userRepo, sessSvc, in.TokenManager, in.ChangeLog, in.Mailer, in.EmailUniqueMu, accountapp.Config{
		AppURL:                 in.Config.Env.AppURL,
		ServerName:             in.Config.App.General.ServerName,
		EmailChangeTokenTTL:    in.Config.App.TTL.EmailChange,
		EmailChangeRequestCool: in.Config.App.Cooldown.EmailChangeRequest,
		EmailChangeCool:        in.Config.App.Cooldown.EmailChange,
		PasswordChangeCool:     in.Config.App.Cooldown.PasswordChange,
	})

	requireLogin := authtransport.RequireLogin(sessSvc, in.Logger)
	h := transport.NewHandler(svc, sessSvc, transport.HandlerConfig{
		Logger:  in.Logger,
		Secure:  in.Config.Env.Mode != "development",
		General: in.Config.App.General,
	})
	h.RegisterRoutes(mux, requireLogin)
}
