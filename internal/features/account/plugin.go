package account

import (
	"fmt"
	"net/http"

	currencyapp "github.com/hayakawakaki/go-racp/internal/features/account/app/currency"
	modapp "github.com/hayakawakaki/go-racp/internal/features/account/app/moderation"
	app "github.com/hayakawakaki/go-racp/internal/features/account/app/self"
	"github.com/hayakawakaki/go-racp/internal/features/account/infra"
	modtransport "github.com/hayakawakaki/go-racp/internal/features/account/transport/moderation"
	transport "github.com/hayakawakaki/go-racp/internal/features/account/transport/self"
	"github.com/hayakawakaki/go-racp/internal/features/character"
	coreinfra "github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/platform/plugin"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
	"github.com/hayakawakaki/go-racp/internal/platform/security"
	"github.com/hayakawakaki/go-racp/internal/platform/theme"
	"github.com/hayakawakaki/go-racp/server/config"
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

	currencyRepo := infra.NewCurrencyRepository(in.DB)
	currencySvc := newCurrencyService(currencyRepo, in.Config.App.Currency, currencyapp.WithBridge(in.MainDB))

	depositWorker := currencyapp.NewDepositWorker(currencyRepo, infra.NewDepositQueue(in.MainDB), currencyapp.DepositWorkerConfig{
		Logger:   in.Logger,
		Interval: in.Config.App.Currency.DepositPollInterval,
		Cooldown: in.Config.App.Currency.Cooldown,
	})
	go depositWorker.Run(in.ShutdownCtx)

	withdrawWorker := currencyapp.NewWithdrawWorker(currencyRepo, infra.NewWithdrawQueue(in.MainDB), currencyapp.WithdrawWorkerConfig{
		Logger:    in.Logger,
		Interval:  in.Config.App.Currency.WithdrawDrainInterval,
		ReapAfter: in.Config.App.Currency.ReapAfter,
	})
	go withdrawWorker.Run(in.ShutdownCtx)

	trustedProxies, err := security.ParseTrustedProxies(in.Config.App.Security.TrustedProxyCIDRs)
	if err != nil {
		panic(fmt.Errorf("account/plugin: %w", err))
	}

	h := transport.NewHandler(svc, sessSvc, transport.HandlerConfig{
		Logger:               in.Logger,
		Users:                userRepo,
		Characters:           charSvc,
		Currency:             currencySvc,
		Theme:                theme.Active,
		Secure:               secure,
		General:              in.Config.App.General,
		AllowTempBannedLogin: in.Config.App.Auth.AllowTempBannedLogin,
		TrustedProxies:       trustedProxies,
	})
	h.RegisterRoutes(reg, mux)

	modSvc := buildModerationService(in, userRepo)
	modH := modtransport.NewHandler(modSvc, modtransport.HandlerConfig{
		Logger:   in.Logger,
		General:  in.Config.App.General,
		Currency: currencySvc,
		Theme:    theme.Active,
	})
	modH.RegisterRoutes(reg, mux)
}

func buildServices(in *coreinfra.Infra) (*app.Service, *app.SessionService, *infra.Repository) {
	userRepo := infra.NewRepository(in.MainDB)
	sessRepo := infra.NewSessionRepository(in.DB)
	changeLog := infra.NewChangeLogRepository(in.DB)
	loginAttempts := infra.NewLoginAttemptsRepository(in.DB)
	sessSvc := app.NewSessionService(sessRepo, in.Config.App.TTL.Session)

	svc := app.NewService(userRepo,
		app.WithLocation(in.Config.App.General.Location()),
		app.WithSessionInvalidator(sessSvc),
		app.WithChangeLog(changeLog),
		app.WithLoginAttempts(loginAttempts),
		app.WithAuthLogger(in.Logger),
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

func BuildUserDirectory(in *coreinfra.Infra) *infra.Repository {
	return infra.NewRepository(in.MainDB)
}

func BuildCurrencyService(in *coreinfra.Infra) *currencyapp.Service {
	return newCurrencyService(infra.NewCurrencyRepository(in.DB), in.Config.App.Currency)
}

func newCurrencyService(repo *infra.CurrencyRepository, cfg config.CurrencyConfig, opts ...currencyapp.Option) *currencyapp.Service {
	base := make([]currencyapp.Option, 0, 3+len(opts))
	base = append(base,
		currencyapp.WithCooldown(cfg.Cooldown),
		currencyapp.WithLimits(cfg.MaxZenyPerTx, cfg.MaxCashpointPerTx),
		currencyapp.WithReapAfter(cfg.ReapAfter),
	)
	base = append(base, opts...)

	return currencyapp.NewService(repo, base...)
}

func BuildModerationService(in *coreinfra.Infra) *modapp.Service {
	userRepo := infra.NewRepository(in.MainDB)

	return buildModerationService(in, userRepo)
}

func buildModerationService(in *coreinfra.Infra, userRepo *infra.Repository) *modapp.Service {
	allowed := map[int]string{0: "Player"}
	for name, groupID := range in.Config.App.UserRoles {
		allowed[groupID] = name
	}

	return modapp.NewService(modapp.Sources{
		Users:        userRepo,
		Characters:   infra.NewCharRepository(in.MainDB),
		Audits:       infra.NewAuditRepository(in.DB),
		AllowedRoles: allowed,
		Logger:       in.Logger,
	})
}
