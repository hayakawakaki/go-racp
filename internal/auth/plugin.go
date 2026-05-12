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
	plugin.Register(plugin.Plugin{
		Name:       "auth",
		Mount:      mount,
		Middleware: middleware,
	})
}

// mount registers authentication HTTP routes on mux.
// It wires user and session repositories and services, creates the transport handler (marked secure unless Env.Mode == "development"), and registers its routes.
func mount(mux *http.ServeMux, in *platinfra.Infra) {
	svc, sessSvc, userRepo := buildServices(in)
	h := transport.NewHandler(svc, sessSvc, transport.HandlerConfig{
		Logger:    in.Logger,
		Users:     userRepo,
		VerifySvc: svc,
		ResetSvc:  svc,
		Secure:    in.Config.Env.Mode != "development",
		General:   in.Config.App.General,
	})
	h.RegisterRoutes(mux)
}

func middleware(in *platinfra.Infra, h http.Handler) http.Handler {
	_, sessSvc, userRepo := buildServices(in)
	allow := []string{
		"/login", "/logout", "/register",
		"/verify-account", "/verify", "/verify/resend",
		"/forgot-password", "/reset-password",
		"/verify-email-change",
		"/healthz", "/static",
	}
	mw := transport.RequireVerified(sessSvc, userRepo, in.Logger, allow)
	return mw(h)
}

func buildServices(in *platinfra.Infra) (*app.Service, *app.SessionService, *infra.Repository) {
	userRepo := infra.NewRepository(in.MainDB)
	sessRepo := infra.NewSessionRepository(in.MainDB)
	sessSvc := app.NewSessionService(sessRepo, in.Config.App.TTL.Session)

	svc := app.NewService(userRepo,
		app.WithEmailUniquenessLock(in.EmailUniqueMu),
		app.WithSessionInvalidator(sessSvc),
		app.WithChangeLog(in.ChangeLog),
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
	)
	return svc, sessSvc, userRepo
}
