package billing

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/hayakawakaki/go-racp/internal/features/account"
	app "github.com/hayakawakaki/go-racp/internal/features/billing/app"
	"github.com/hayakawakaki/go-racp/internal/features/billing/domain"
	infra "github.com/hayakawakaki/go-racp/internal/features/billing/infra"
	"github.com/hayakawakaki/go-racp/internal/features/billing/transport"
	coreinfra "github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/platform/plugin"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
	"github.com/hayakawakaki/go-racp/internal/platform/theme"
)

var (
	svcOnce     sync.Once
	svcInstance *app.Service
)

func init() {
	plugin.Register(plugin.Plugin{Name: "billing", Mount: mount})
}

func BuildService(in *coreinfra.Infra) *app.Service {
	svcOnce.Do(func() {
		repo := infra.NewPurchaseRepository(in.DB)
		banner := infra.NewChargebackBanner(account.BuildModerationService(in))
		svcInstance = app.NewService(repo, buildCatalog(in),
			app.WithBanner(banner),
			app.WithLogger(in.Logger),
			app.WithLocation(in.Config.App.General.Location()),
		)
	})

	return svcInstance
}

func buildCatalog(in *coreinfra.Infra) domain.Catalog {
	purchases := in.Config.App.Purchases
	if len(purchases.Packages) > 0 && !domain.IsSupportedCurrency(purchases.Currency) {
		panic(fmt.Errorf("billing: Purchases.Currency %q is not supported, must be one of %v", purchases.Currency, domain.SupportedCurrencies()))
	}

	pkgs := make([]domain.Package, 0, len(purchases.Packages))
	for _, pkg := range purchases.Packages {
		pkgs = append(pkgs, domain.Package{
			Key:        pkg.Key,
			Name:       pkg.Name,
			Currency:   purchases.Currency,
			Price:      pkg.Price,
			CashPoints: pkg.CashPoints,
		})
	}

	return domain.NewCatalog(pkgs)
}

func mount(reg *routes.Registry, mux *http.ServeMux, in *coreinfra.Infra) {
	svc := BuildService(in)

	isProd := in.Config.Env.Mode != "development"
	webhookSecret := registerStripeProvider(in, isProd)
	paypalClient := registerPaypalProvider(in, isProd)

	cfg := transport.HandlerConfig{
		Logger:              in.Logger,
		Theme:               theme.Active,
		General:             in.Config.App.General,
		Currency:            in.Config.App.Purchases.Currency,
		AppURL:              in.Config.Env.AppURL,
		StripeWebhookSecret: webhookSecret,
	}
	if paypalClient != nil {
		cfg.Paypal = paypalClient
		cfg.PaypalWebhookID = in.Config.Env.PaypalWebhookID
	}

	h := transport.NewHandler(svc, cfg)
	h.RegisterRoutes(reg, mux)
}

func registerStripeProvider(in *coreinfra.Infra, isProd bool) string {
	if !in.Config.App.Purchases.Providers.Stripe {
		return ""
	}

	switch {
	case in.Config.Env.StripeSecretKey == "":
		in.Logger.Warn("payment provider stripe enabled but STRIPE_SECRET_KEY is unset, checkouts disabled")
	case in.Config.Env.StripeWebhookSecret == "":
		in.Logger.Warn("payment provider stripe enabled but STRIPE_WEBHOOK_SECRET is unset, checkouts disabled to avoid uncredited payments")
	case isProd && strings.Contains(in.Config.Env.StripeSecretKey, "_test_"):
		in.Logger.Warn("payment provider stripe is using a test secret key in production mode, checkouts disabled")
	default:
		SetProvider(infra.NewStripeProvider(in.Config.Env.StripeSecretKey))
		return in.Config.Env.StripeWebhookSecret
	}

	return ""
}

func registerPaypalProvider(in *coreinfra.Infra, isProd bool) *infra.PaypalClient {
	if !in.Config.App.Purchases.Providers.Paypal {
		return nil
	}

	switch {
	case in.Config.Env.PaypalClientID == "":
		in.Logger.Warn("payment provider paypal enabled but PAYPAL_CLIENT_ID is unset, checkouts disabled")
		return nil
	case in.Config.Env.PaypalSecret == "":
		in.Logger.Warn("payment provider paypal enabled but PAYPAL_SECRET is unset, checkouts disabled")
		return nil
	case in.Config.Env.PaypalWebhookID == "":
		in.Logger.Warn("payment provider paypal enabled but PAYPAL_WEBHOOK_ID is unset, checkouts disabled to avoid uncredited payments")
		return nil
	default:
		client := infra.NewPaypalClient(in.Config.Env.PaypalClientID, in.Config.Env.PaypalSecret, isProd)
		SetProvider(infra.NewPaypalProvider(client))

		return client
	}
}

func SetProvider(provider domain.Provider) {
	if svcInstance != nil {
		svcInstance.SetProvider(provider)
	}
}
