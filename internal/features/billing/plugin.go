package billing

import (
	"net/http"
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

	if in.Config.App.Purchases.Providers.Stripe {
		if in.Config.Env.StripeSecretKey != "" {
			SetProvider(infra.NewStripeProvider(in.Config.Env.StripeSecretKey))
		} else {
			in.Logger.Warn("payment provider stripe enabled but STRIPE_SECRET_KEY is unset")
		}
	}

	h := transport.NewHandler(svc, transport.HandlerConfig{
		Logger:              in.Logger,
		Theme:               theme.Active,
		General:             in.Config.App.General,
		Currency:            in.Config.App.Purchases.Currency,
		AppURL:              in.Config.Env.AppURL,
		StripeWebhookSecret: in.Config.Env.StripeWebhookSecret,
	})
	h.RegisterRoutes(reg, mux)
}

func SetProvider(provider domain.Provider) {
	if svcInstance != nil {
		svcInstance.SetProvider(provider)
	}
}
