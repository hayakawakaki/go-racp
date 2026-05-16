package character

import (
	"net/http"

	characterapp "github.com/hayakawakaki/go-racp/internal/character/app"
	characterinfra "github.com/hayakawakaki/go-racp/internal/character/infra"
	charactertransport "github.com/hayakawakaki/go-racp/internal/character/transport"
	platinfra "github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/plugin"
	"github.com/hayakawakaki/go-racp/internal/routes"
)

func init() {
	plugin.Register(plugin.Plugin{Name: "character", Mount: mount})
}

func mount(reg *routes.Registry, mux *http.ServeMux, in *platinfra.Infra) {
	svc := BuildService(in)
	handler := charactertransport.NewHandler(svc, charactertransport.HandlerConfig{
		Logger:  in.Logger,
		General: in.Config.App.General,
	})
	handler.RegisterRoutes(reg, mux)
}

func BuildService(in *platinfra.Infra) *characterapp.Service {
	repo := characterinfra.NewRepository(in.MainDB)
	cooldowns := characterinfra.NewCooldownRepository(in.DB)

	return characterapp.NewService(repo, cooldowns,
		characterapp.WithCooldowns(in.Config.App.Cooldown.CharacterLookReset, in.Config.App.Cooldown.CharacterLocationReset),
		characterapp.WithDefaultLocation(characterapp.DefaultLocation{
			Map: in.Config.App.DefaultLocation.Map,
			X:   in.Config.App.DefaultLocation.X,
			Y:   in.Config.App.DefaultLocation.Y,
		}),
	)
}
