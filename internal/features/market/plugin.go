package market

import (
	"net/http"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/market/app"
	"github.com/hayakawakaki/go-racp/internal/features/market/infra"
	"github.com/hayakawakaki/go-racp/internal/features/market/transport"
	coreinfra "github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/platform/plugin"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
	"github.com/hayakawakaki/go-racp/internal/platform/worker"
)

func init() {
	plugin.Register(plugin.Plugin{Name: "market", Mount: mount})
}

func mount(reg *routes.Registry, mux *http.ServeMux, in *coreinfra.Infra) {
	stashRepo := infra.NewStashRepository(in.MainDB)
	escrowRepo := infra.NewEscrowRepository(in.MainDB, 0, 0)
	walletRepo := infra.NewWalletRepository(in.DB)
	listingRepo := infra.NewListingRepository(in.DB)
	settlementRepo := infra.NewSettlementRepository(in.DB)

	stashService := app.NewStashService(stashRepo, 0)
	offerService := app.NewOfferService(listingRepo, stashRepo, escrowRepo, walletRepo, settlementRepo, in.Logger)

	handler := transport.NewHandler(stashService, offerService, in.Logger)
	handler.RegisterRoutes(reg, mux)

	workers := app.NewWorkers(listingRepo, escrowRepo, walletRepo, settlementRepo, in.Logger)
	go worker.Run(in.ShutdownCtx, in.Logger,
		worker.Job{Name: "market-deliver", Interval: 5 * time.Second, Fn: workers.Deliver},
		worker.Job{Name: "market-expire", Interval: time.Minute, Fn: workers.Expire},
		worker.Job{Name: "market-reconcile", Interval: 2 * time.Minute, Fn: workers.Reconcile},
	)
}
