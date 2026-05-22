package character

import (
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/features/character/app"
	"github.com/hayakawakaki/go-racp/internal/features/character/infra"
	"github.com/hayakawakaki/go-racp/internal/features/character/transport"
	coreinfra "github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/platform/plugin"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
	"github.com/hayakawakaki/go-racp/internal/platform/theme"
)

func init() {
	plugin.Register(plugin.Plugin{Name: "character", Mount: mount})
}

func mount(reg *routes.Registry, mux *http.ServeMux, in *coreinfra.Infra) {
	svc := BuildService(in)
	handler := transport.NewHandler(svc, transport.HandlerConfig{
		Logger:  in.Logger,
		General: in.Config.App.General,
		Theme:   theme.Active,
	})
	handler.RegisterRoutes(reg, mux)
}

func BuildService(in *coreinfra.Infra) *app.Service {
	repo := infra.NewRepository(in.MainDB)
	cooldowns := infra.NewCooldownRepository(in.DB)

	return app.NewService(repo, cooldowns,
		app.WithCooldowns(in.Config.App.Cooldown.CharacterLookReset, in.Config.App.Cooldown.CharacterLocationReset),
		app.WithDefaultLocation(app.DefaultLocation{
			Map: in.Config.App.DefaultLocation.Map,
			X:   in.Config.App.DefaultLocation.X,
			Y:   in.Config.App.DefaultLocation.Y,
		}),
	)
}
