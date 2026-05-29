package admin

import (
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/features/account"
	"github.com/hayakawakaki/go-racp/internal/features/admin/transport"
	"github.com/hayakawakaki/go-racp/internal/features/guild"
	"github.com/hayakawakaki/go-racp/internal/features/item"
	"github.com/hayakawakaki/go-racp/internal/features/mob"
	coreinfra "github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/platform/plugin"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
	"github.com/hayakawakaki/go-racp/internal/platform/theme"
)

func init() {
	plugin.Register(plugin.Plugin{
		Name:  "admin",
		Mount: mount,
	})
}

func mount(reg *routes.Registry, mux *http.ServeMux, in *coreinfra.Infra) {
	h := transport.NewHandler(transport.HandlerConfig{
		Logger:     in.Logger,
		General:    in.Config.App.General,
		ItemStatus: item.BuildService(in),
		MobStatus:  mob.BuildService(in),
		Metric:     in.Metric,
		Users:      account.BuildModerationService(in),
		Guilds:     guild.BuildService(in),
		Economy:    account.BuildCurrencyService(in),
		Emails:     account.BuildUserDirectory(in),
		Theme:      theme.Active,
	})
	h.RegisterRoutes(reg, mux)
}
