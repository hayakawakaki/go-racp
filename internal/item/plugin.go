package item

import (
	"log/slog"
	"net/http"

	platinfra "github.com/hayakawakaki/go-racp/internal/infra"
	itemapp "github.com/hayakawakaki/go-racp/internal/item/app"
	"github.com/hayakawakaki/go-racp/internal/item/domain"
	itemtransport "github.com/hayakawakaki/go-racp/internal/item/transport"
	"github.com/hayakawakaki/go-racp/internal/plugin"
	"github.com/hayakawakaki/go-racp/internal/routes"
	"github.com/hayakawakaki/go-racp/server/config"
)

func init() {
	plugin.Register(plugin.Plugin{Name: "item", Mount: mount})
}

func mount(reg *routes.Registry, mux *http.ServeMux, in *platinfra.Infra) {
	service := BuildService(in)
	handler := itemtransport.NewHandler(service, itemtransport.HandlerConfig{
		Logger:  in.Logger,
		General: in.Config.App.General,
	})
	handler.RegisterRoutes(reg, mux)
}

func BuildService(in *platinfra.Infra) *itemapp.Service {
	loader := func() (*domain.Snapshot, error) {
		return itemapp.ParseSources(buildSources(in.Config.App.ItemDB, in.Logger))
	}
	snap, err := loader()
	if err != nil {
		in.Logger.Error("item: initial load failed", "err", err)
		panic(err)
	}
	if snap.SourceCount == 0 {
		in.Logger.Warn("item: no item database configured, serving empty snapshot")
	} else {
		in.Logger.Info("item: snapshot loaded", "items", snap.SourceCount)
	}

	return itemapp.NewServiceWithSnapshot(snap, loader)
}

func buildSources(cfg config.ItemDBConfig, logger *slog.Logger) itemapp.Sources {
	return itemapp.Sources{
		Logger: logger,
		Root:   cfg.Root,
		YAML:   cfg.YAML,
		Lua:    cfg.Lua,
	}
}
