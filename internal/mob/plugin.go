package mob

import (
	"context"
	"log/slog"
	"net/http"
	"path/filepath"

	platinfra "github.com/hayakawakaki/go-racp/internal/infra"
	mobapp "github.com/hayakawakaki/go-racp/internal/mob/app"
	"github.com/hayakawakaki/go-racp/internal/mob/domain"
	mobtransport "github.com/hayakawakaki/go-racp/internal/mob/transport"
	"github.com/hayakawakaki/go-racp/internal/plugin"
	"github.com/hayakawakaki/go-racp/internal/refdata"
	"github.com/hayakawakaki/go-racp/internal/routes"
	"github.com/hayakawakaki/go-racp/server/config"
)

const (
	devCacheSubdir   = "tmp"
	mobCacheFileName = "mob-snapshot.gob"
)

func init() {
	plugin.Register(plugin.Plugin{Name: "mob", Mount: mount})
}

func mount(reg *routes.Registry, mux *http.ServeMux, in *platinfra.Infra) {
	service := BuildService(in)
	handler := mobtransport.NewHandler(service, mobtransport.HandlerConfig{
		Logger:  in.Logger,
		General: in.Config.App.General,
	})
	handler.RegisterRoutes(reg, mux)
	startDevWatcher(in, service)
}

func BuildService(in *platinfra.Infra) *mobapp.Service {
	sources := buildSources(in)
	loader := func() (*domain.Snapshot, error) {
		return mobapp.ParseSources(sources)
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

	return mobapp.NewServiceWithSnapshot(snap, loader)
}

func buildSources(in *platinfra.Infra) mobapp.Sources {
	cfg := in.Config.App.MobDB
	sources := mobapp.Sources{
		Logger: in.Logger,
		YAML:   cfg.YAML,
	}
	if dir := devCacheDir(in.Config.Env.Mode, in.Logger); dir != "" {
		sources.Cache = &mobapp.MobCache{
			Logger:   in.Logger,
			Dir:      dir,
			Filename: mobCacheFileName,
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
		logger.Warn("mob: cache disabled, project root not found", "err", err)

		return ""
	}

	return filepath.Join(root, devCacheSubdir)
}

func startDevWatcher(in *platinfra.Infra, service *mobapp.Service) {
	if in.Config.Env.Mode != "development" {
		return
	}
	paths, err := mobapp.ResolveSourcePaths(buildSources(in))
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
