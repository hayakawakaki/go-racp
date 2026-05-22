package stall

import (
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/features/item"
	"github.com/hayakawakaki/go-racp/internal/features/stall/app"
	"github.com/hayakawakaki/go-racp/internal/features/stall/infra"
	"github.com/hayakawakaki/go-racp/internal/features/stall/transport"
	coreinfra "github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/platform/plugin"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
	"github.com/hayakawakaki/go-racp/internal/platform/theme"
)

func init() {
	plugin.Register(plugin.Plugin{Name: "stall", Mount: mount})
}

func mount(reg *routes.Registry, mux *http.ServeMux, in *coreinfra.Infra) {
	repo := infra.NewRepository(in.MainDB)
	poller := app.NewPoller(repo, in.Config.App.Vendor.PollInterval, in.Logger)
	go poller.Run(in.ShutdownCtx)

	svc := app.NewService(poller)
	handler := transport.NewHandler(svc, transport.HandlerConfig{
		Logger:     in.Logger,
		ItemLookup: item.BuildService(in),
		General:    in.Config.App.General,
		Theme:      theme.Active,
	})
	handler.RegisterRoutes(reg, mux)
}
