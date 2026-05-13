package account

import (
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/account/app"
	"github.com/hayakawakaki/go-racp/internal/account/domain"
	"github.com/hayakawakaki/go-racp/internal/account/infra"
	"github.com/hayakawakaki/go-racp/internal/account/transport"
	"github.com/hayakawakaki/go-racp/internal/account/transport/middleware"
	platinfra "github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/plugin"
)

func init() {
	plugin.Register(plugin.Plugin{
		Name:       "account",
		Mount:      mount,
		Middleware: requireVerified,
	})
}

func mount(mux *http.ServeMux, in *platinfra.Infra) {
	svc, sessSvc, userRepo := buildServices(in)
	secure := in.Config.Env.Mode != "development"
	requireLogin := middleware.RequireRole(sessSvc, userRepo, in.Roles, in.Logger, secure, domain.RoleAny)

	h := transport.NewHandler(svc, sessSvc, transport.HandlerConfig{
		Logger:  in.Logger,
		Users:   userRepo,
		Secure:  secure,
		General: in.Config.App.General,
	})
	h.RegisterRoutes(mux, requireLogin)
}

func requireVerified(in *platinfra.Infra, h http.Handler) http.Handler {
	_, sessSvc, userRepo := buildServices(in)
	allow := []string{
		"/login", "/logout", "/register",
		"/verify-account", "/verify", "/verify/resend",
		"/forgot-password", "/reset-password",
		"/verify-email-change",
		"/healthz", "/static",
	}

	return middleware.RequireVerified(sessSvc, userRepo, in.Logger, allow)(h)
}

func buildServices(in *platinfra.Infra) (*app.Service, *app.SessionService, *infra.Repository) {
	userRepo := infra.NewRepository(in.MainDB)
	sessRepo := infra.NewSessionRepository(in.MainDB)
	changeLog := infra.NewChangeLogRepository(in.MainDB)
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
