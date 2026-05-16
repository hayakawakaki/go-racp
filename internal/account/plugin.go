package account

import (
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/account/app"
	"github.com/hayakawakaki/go-racp/internal/account/infra"
	"github.com/hayakawakaki/go-racp/internal/account/transport"
	platinfra "github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/plugin"
	"github.com/hayakawakaki/go-racp/internal/routes"
)

func init() {
	plugin.Register(plugin.Plugin{
		Name:  "account",
		Mount: mount,
	})
}

func mount(reg *routes.Registry, mux *http.ServeMux, in *platinfra.Infra) {
	svc, sessSvc, userRepo := buildServices(in)
	secure := in.Config.Env.Mode != "development"

	h := transport.NewHandler(svc, sessSvc, transport.HandlerConfig{
		Logger:  in.Logger,
		Users:   userRepo,
		Secure:  secure,
		General: in.Config.App.General,
	})
	h.RegisterRoutes(reg, mux)
}

func buildServices(in *platinfra.Infra) (*app.Service, *app.SessionService, *infra.Repository) {
	userRepo := infra.NewRepository(in.MainDB)
	sessRepo := infra.NewSessionRepository(in.DB)
	changeLog := infra.NewChangeLogRepository(in.DB)
	sessSvc := app.NewSessionService(sessRepo, in.Config.App.TTL.Session)

	svc := app.NewService(userRepo,
		app.WithLocation(in.Config.App.General.Location()),
		app.WithSessionInvalidator(sessSvc),
		app.WithChangeLog(changeLog),
		app.WithVerification(in.TokenManager, in.Mailer, app.VerificationConfig{
			AppURL:         in.Config.Env.AppURL,
			ServerName:     in.Config.App.General.ServerName,
			TokenTTL:       in.Config.App.TTL.Verification,
			ResendCooldown: in.Config.App.Cooldown.VerificationResend,
		}),
		app.WithPasswordReset(in.TokenManager, in.Mailer, app.PasswordResetConfig{
			AppURL:         in.Config.Env.AppURL,
			ServerName:     in.Config.App.General.ServerName,
			TokenTTL:       in.Config.App.TTL.PasswordReset,
			ResendCooldown: in.Config.App.Cooldown.PasswordResetRequest,
			ChangeCooldown: in.Config.App.Cooldown.PasswordChange,
		}),
		app.WithEmailChange(in.TokenManager, in.Mailer, app.EmailChangeConfig{
			AppURL:           in.Config.Env.AppURL,
			ServerName:       in.Config.App.General.ServerName,
			TokenTTL:         in.Config.App.TTL.EmailChange,
			RequestCooldown:  in.Config.App.Cooldown.EmailChangeRequest,
			ChangeCooldown:   in.Config.App.Cooldown.EmailChange,
			PasswordCooldown: in.Config.App.Cooldown.PasswordChange,
		}),
	)

	return svc, sessSvc, userRepo
}
