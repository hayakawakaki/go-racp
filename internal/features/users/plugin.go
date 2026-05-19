package users

import (
	"net/http"

	usersapp "github.com/hayakawakaki/go-racp/internal/features/users/app"
	"github.com/hayakawakaki/go-racp/internal/features/users/infra"
	userstransport "github.com/hayakawakaki/go-racp/internal/features/users/transport"
	platinfra "github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/plugin"
	"github.com/hayakawakaki/go-racp/internal/routes"
)

func init() {
	plugin.Register(plugin.Plugin{Name: "users", Mount: mount})
}

func mount(reg *routes.Registry, mux *http.ServeMux, in *platinfra.Infra) {
	allowed := buildAllowedRoles(in)

	service := usersapp.NewService(usersapp.Sources{
		Users:        infra.NewUserRepository(in.MainDB),
		Characters:   infra.NewCharRepository(in.MainDB),
		Actions:      infra.NewActionRepository(in.DB),
		AllowedRoles: allowed,
		Logger:       in.Logger,
	})

	handler := userstransport.NewHandler(service, userstransport.HandlerConfig{
		Logger:  in.Logger,
		General: in.Config.App.General,
	})
	handler.RegisterRoutes(reg, mux)
}

func buildAllowedRoles(in *platinfra.Infra) map[int]string {
	out := map[int]string{0: "Player"}
	for name, groupID := range in.Config.App.UserRoles {
		out[groupID] = name
	}

	return out
}
