package users

import (
	"net/http"

	platinfra "github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/plugin"
	"github.com/hayakawakaki/go-racp/internal/routes"
	usersapp "github.com/hayakawakaki/go-racp/internal/users/app"
	usersinfra "github.com/hayakawakaki/go-racp/internal/users/infra"
	userstransport "github.com/hayakawakaki/go-racp/internal/users/transport"
)

func init() {
	plugin.Register(plugin.Plugin{Name: "users", Mount: mount})
}

func mount(reg *routes.Registry, mux *http.ServeMux, in *platinfra.Infra) {
	allowed := buildAllowedRoles(in)

	service := usersapp.NewService(usersapp.Sources{
		Users:        usersinfra.NewUserRepository(in.MainDB),
		Characters:   usersinfra.NewCharRepository(in.MainDB),
		Actions:      usersinfra.NewActionRepository(in.DB),
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
