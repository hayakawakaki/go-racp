package billing

import (
	"net/http"

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

var svcInstance *app.Service

func init() {
	plugin.Register(plugin.Plugin{Name: "billing", Mount: mount})
}

func mount(reg *routes.Registry, mux *http.ServeMux, in *coreinfra.Infra) {
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
	catalog := domain.NewCatalog(pkgs)

	repo := infra.NewPurchaseRepository(in.DB)
	banner := infra.NewChargebackBanner(account.BuildModerationService(in))
	svc := app.NewService(repo, catalog, app.WithBanner(banner), app.WithLogger(in.Logger))
	svcInstance = svc

	h := transport.NewHandler(svc, transport.HandlerConfig{
		Logger:   in.Logger,
		Theme:    theme.Active,
		General:  in.Config.App.General,
		Currency: purchases.Currency,
		AppURL:   in.Config.Env.AppURL,
	})
	h.RegisterRoutes(reg, mux)
}

func SetProvider(provider domain.Provider) {
	if svcInstance != nil {
		svcInstance.SetProvider(provider)
	}
}
