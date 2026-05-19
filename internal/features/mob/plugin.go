package mob

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/hayakawakaki/go-racp/internal/features/mob/app"
	"github.com/hayakawakaki/go-racp/internal/features/mob/domain"
	"github.com/hayakawakaki/go-racp/internal/features/mob/transport"
	coreinfra "github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/platform/refdata"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
	"github.com/hayakawakaki/go-racp/internal/plugin"
)

const mobCacheFileName = "mob-snapshot.gob"

var (
	svcOnce          sync.Once
	svcInstance      *app.Service
	itemLookupAtomic atomic.Pointer[transport.ItemLookup]
)

func SetItemLookup(l transport.ItemLookup) {
	itemLookupAtomic.Store(&l)
}

func currentItemLookup() transport.ItemLookup {
	p := itemLookupAtomic.Load()
	if p == nil {
		return nil
	}

	return *p
}

func init() {
	plugin.Register(plugin.Plugin{Name: "mob", Mount: mount})
}

func mount(reg *routes.Registry, mux *http.ServeMux, in *coreinfra.Infra) {
	service := BuildService(in)
	handler := transport.NewHandler(service, transport.HandlerConfig{
		Logger:       in.Logger,
		General:      in.Config.App.General,
		ItemLookupFn: currentItemLookup,
	})
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
			in.Logger.Error("mob: initial load failed", "err", err)
			panic(err)
		}
		if snap.SourceCount == 0 {
			in.Logger.Warn("mob: no monster database configured, serving empty snapshot")
		} else {
			in.Logger.Info("mob: snapshot loaded", "mobs", snap.SourceCount)
		}

		svcInstance = app.NewServiceWithSnapshot(snap, loader)
	})

	return svcInstance
}

func buildSources(in *coreinfra.Infra) app.Sources {
	cfg := in.Config.App.MobDB
	sources := app.Sources{
		Logger: in.Logger,
		YAML:   cfg.YAML,
	}
	if dir := coreinfra.DevCacheDir(in.Config.Env.Mode, in.Logger); dir != "" {
		sources.Cache = &app.MobCache{
			Logger:   in.Logger,
			Dir:      dir,
			Filename: mobCacheFileName,
		}
	}

	return sources
}

func startDevWatcher(in *coreinfra.Infra, service *app.Service) {
	if in.Config.Env.Mode != "development" {
		return
	}
	paths, err := app.ResolveSourcePaths(buildSources(in))
	if err != nil {
		in.Logger.Warn("mob: dev watcher disabled, cannot resolve sources", "err", err)
		return
	}
	if len(paths) == 0 {
		return
	}
	if _, err := refdata.StartWatcher(context.Background(), paths, service.Reload, in.Logger); err != nil {
		in.Logger.Warn("mob: dev watcher failed to start", "err", err)
		return
	}
	in.Logger.Info("mob: dev watcher started", "files", len(paths))
}
