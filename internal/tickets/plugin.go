package tickets

import (
	"fmt"
	"net/http"

	platinfra "github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/plugin"
	"github.com/hayakawakaki/go-racp/internal/routes"
	"github.com/hayakawakaki/go-racp/server/config"
)

func init() {
	plugin.Register(plugin.Plugin{
		Name:  "tickets",
		Mount: mount,
	})
}

func mount(reg *routes.Registry, mux *http.ServeMux, in *platinfra.Infra) {
	validateAccessCrossConfig(in.Config.App.TicketCategories, config.ProcessAccessConfig())
	_ = reg
	_ = mux
}

func validateAccessCrossConfig(categories config.TicketCategoriesConfig, access config.AccessConfig) {
	staffAccess, ok := access["Tickets"]
	if !ok {
		return
	}
	list, ok := staffAccess["StaffAccess"]
	if !ok {
		return
	}
	allowed := make(map[string]struct{}, len(list))
	for _, role := range list {
		allowed[role] = struct{}{}
	}

	for key, category := range categories {
		for _, role := range category.Roles {
			if role == "*" || role == "Admin" {
				continue
			}
			if _, ok := allowed[role]; !ok {
				panic(fmt.Errorf("TicketCategories.%s lists role %q but Tickets.StaffAccess does not — that role can never reach the category", key, role))
			}
		}
	}
}
