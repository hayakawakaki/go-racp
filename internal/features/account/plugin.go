package account

import (
	"net/http"

	modapp "github.com/hayakawakaki/go-racp/internal/features/account/app/moderation"
	app "github.com/hayakawakaki/go-racp/internal/features/account/app/self"
	"github.com/hayakawakaki/go-racp/internal/features/account/infra"
	modtransport "github.com/hayakawakaki/go-racp/internal/features/account/transport/moderation"
	transport "github.com/hayakawakaki/go-racp/internal/features/account/transport/self"
	"github.com/hayakawakaki/go-racp/internal/features/character"
	coreinfra "github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/platform/plugin"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
)

func init() {
	plugin.Register(plugin.Plugin{
		Name:  "account",
		Mount: mount,
	})
}

func mount(reg *routes.Registry, mux *http.ServeMux, in *coreinfra.Infra) {
	svc, sessSvc, userRepo := buildServices(in)
	secure := in.Config.Env.Mode != "development"

	charSvc := character.BuildService(in)

	h := transport.NewHandler(svc, sessSvc, transport.HandlerConfig{
		Logger:               in.Logger,
		Users:                userRepo,
		Characters:           charSvc,
		Secure:               secure,
		General:              in.Config.App.General,
		AllowTempBannedLogin: in.Config.App.Auth.AllowTempBannedLogin,
	})
	h.RegisterRoutes(reg, mux)

	modSvc := buildModerationService(in, userRepo)
	modH := modtransport.NewHandler(modSvc, modtransport.HandlerConfig{
		Logger:  in.Logger,
		General: in.Config.App.General,
	})
	modH.RegisterRoutes(reg, mux)
}

func buildServices(in *coreinfra.Infra) (*app.Service, *app.SessionService, *infra.Repository) {
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

func buildModerationService(in *coreinfra.Infra, userRepo *infra.Repository) *modapp.Service {
	allowed := map[int]string{0: "Player"}
	for name, groupID := range in.Config.App.UserRoles {
		allowed[groupID] = name
	}

	return modapp.NewService(modapp.Sources{
		Users:        userRepo,
		Characters:   infra.NewCharRepository(in.MainDB),
		Actions:      infra.NewActionRepository(in.DB),
		AllowedRoles: allowed,
		Logger:       in.Logger,
	})
}
