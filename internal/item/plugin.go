package item

import (
	"context"
	"log/slog"
	"net/http"
	"path/filepath"
	"slices"

	platinfra "github.com/hayakawakaki/go-racp/internal/infra"
	itemapp "github.com/hayakawakaki/go-racp/internal/item/app"
	"github.com/hayakawakaki/go-racp/internal/item/domain"
	itemtransport "github.com/hayakawakaki/go-racp/internal/item/transport"
	"github.com/hayakawakaki/go-racp/internal/plugin"
	"github.com/hayakawakaki/go-racp/internal/refdata"
	"github.com/hayakawakaki/go-racp/internal/routes"
	"github.com/hayakawakaki/go-racp/server/config"
)

const (
	devCacheSubdir    = "tmp"
	itemCacheFileName = "item-snapshot.gob"
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
	startDevWatcher(in, service)
}

func BuildService(in *platinfra.Infra) *itemapp.Service {
	sources := buildSources(in)
	loader := func() (*domain.Snapshot, error) {
		return itemapp.ParseSources(sources)
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

func buildSources(in *platinfra.Infra) itemapp.Sources {
	cfg := in.Config.App.ItemDB
	sources := itemapp.Sources{
		Logger: in.Logger,
		YAML:   cfg.YAML,
		Lua:    cfg.Lua,
	}
	if dir := devCacheDir(in.Config.Env.Mode, in.Logger); dir != "" {
		sources.Cache = &itemapp.ItemCache{
			Logger:   in.Logger,
			Dir:      dir,
			Filename: itemCacheFileName,
		}
	}

	return sources
}

func devCacheDir(mode string, logger *slog.Logger) string {
	if mode != "development" {
		return ""
	}
	root, err := config.ProjectRoot()
	if err != nil {
		logger.Warn("item: cache disabled, project root not found", "err", err)

		return ""
	}

	return filepath.Join(root, devCacheSubdir)
}

func startDevWatcher(in *platinfra.Infra, service *itemapp.Service) {
	if in.Config.Env.Mode != "development" {
		return
	}
	yamlPaths, luaPaths, err := itemapp.ResolveSourcePaths(buildSources(in))
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
