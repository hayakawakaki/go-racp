package item

import (
	"context"
	"net/http"
	"slices"
	"sync"

	"github.com/hayakawakaki/go-racp/internal/features/item/app"
	"github.com/hayakawakaki/go-racp/internal/features/item/domain"
	"github.com/hayakawakaki/go-racp/internal/features/item/transport"
	"github.com/hayakawakaki/go-racp/internal/features/mob"
	coreinfra "github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/platform/plugin"
	"github.com/hayakawakaki/go-racp/internal/platform/refdata"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
	"github.com/hayakawakaki/go-racp/internal/platform/theme"
	"github.com/hayakawakaki/go-racp/internal/platform/ui"
)

const itemCacheFileName = "item-snapshot.gob"

var (
	svcOnce     sync.Once
	svcInstance *app.Service
)

func init() {
	plugin.Register(plugin.Plugin{Name: "item", Mount: mount})
}

func mount(reg *routes.Registry, mux *http.ServeMux, in *coreinfra.Infra) {
	service := BuildService(in)
	mob.SetItemLookup(service)

	handler := transport.NewHandler(service, transport.HandlerConfig{
		Logger:     in.Logger,
		General:    in.Config.App.General,
		DropLookup: mob.BuildService(in),
		Theme:      theme.Active,
	})

	// Item sprite component helper
	ui.Sprites.Item = func(id int) string {
		item, err := service.Get(context.Background(), id)
		if err != nil || item == nil {
			return ""
		}

		return item.Image
	}

	handler.RegisterRoutes(reg, mux)
	startDevWatcher(in, service)
}

func BuildService(in *coreinfra.Infra) *app.Service {
	svcOnce.Do(func() {
		sources := buildSources(in)
		loader := func() (*domain.Snapshot, error) {
			return app.ParseSources(sources)
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

		svcInstance = app.NewServiceWithSnapshot(snap, loader)
	})

	return svcInstance
}

func buildSources(in *coreinfra.Infra) app.Sources {
	cfg := in.Config.App.ItemDB
	sources := app.Sources{
		Logger: in.Logger,
		YAML:   cfg.YAML,
		Lua:    cfg.Lua,
	}
	if dir := coreinfra.DevCacheDir(in.Config.Env.Mode, in.Logger); dir != "" {
		sources.Cache = &app.ItemCache{
			Logger:   in.Logger,
			Dir:      dir,
			Filename: itemCacheFileName,
		}
	}

	return sources
}

func startDevWatcher(in *coreinfra.Infra, service *app.Service) {
	if in.Config.Env.Mode != "development" {
		return
	}
	yamlPaths, luaPaths, err := app.ResolveSourcePaths(buildSources(in))
	if err != nil {
		in.Logger.Warn("item: dev watcher disabled, cannot resolve sources", "err", err)
		return
	}
	paths := slices.Concat(yamlPaths, luaPaths)
	if len(paths) == 0 {
		return
	}
	if _, err := refdata.StartWatcher(context.Background(), paths, service.Reload, in.Logger); err != nil {
		in.Logger.Warn("item: dev watcher failed to start", "err", err)
		return
	}
	in.Logger.Info("item: dev watcher started", "files", len(paths))
}
