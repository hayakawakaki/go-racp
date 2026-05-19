package tickets

import (
	"fmt"
	"net/http"

	accinfra "github.com/hayakawakaki/go-racp/internal/features/account/infra"
	ticketsapp "github.com/hayakawakaki/go-racp/internal/features/tickets/app"
	"github.com/hayakawakaki/go-racp/internal/features/tickets/domain"
	"github.com/hayakawakaki/go-racp/internal/features/tickets/infra"
	ticketstransport "github.com/hayakawakaki/go-racp/internal/features/tickets/transport"
	coreinfra "github.com/hayakawakaki/go-racp/internal/infra"
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

func mount(reg *routes.Registry, mux *http.ServeMux, in *coreinfra.Infra) {
	access := config.ProcessAccessConfig()
	validateAccessCrossConfig(in.Config.App.TicketCategories, access)

	categories := buildCategoryResolver(in.Config.App.TicketCategories)
	userRepo := accinfra.NewRepository(in.MainDB)

	ticketRepo := infra.NewRepository(in.DB)
	messageRepo := infra.NewMessageRepository(in.DB)
	viewRepo := infra.NewViewRepository(in.DB)

	service := ticketsapp.NewService(
		ticketRepo, messageRepo, viewRepo, categories, userRepo, in.Mailer,
		in.Logger,
		ticketsapp.Config{
			AppURL:             in.Config.Env.AppURL,
			ServerName:         in.Config.App.General.ServerName,
			MaxOpenPerPlayer:   in.Config.App.TicketLimits.MaxOpenPerPlayer,
			TicketOpenCooldown: in.Config.App.Cooldown.TicketOpen,
		},
	)

	handler := ticketstransport.NewHandler(service, ticketstransport.HandlerConfig{
		Logger:       in.Logger,
		Users:        userRepo,
		Roles:        in.Roles,
		ManageRoles:  access.ManageRoles("Tickets"),
		General:      in.Config.App.General,
		PollInterval: in.Config.App.Tickets.StaffPollInterval,
	})
	handler.RegisterRoutes(reg, mux)
}

func buildCategoryResolver(cfg config.TicketCategoriesConfig) domain.CategoryResolver {
	list := make([]domain.Category, 0, len(cfg))
	for key, cat := range cfg {
		list = append(list, domain.Category{Key: key, Display: cat.Display, Roles: cat.Roles})
	}

	return domain.NewCategoryResolver(list)
}

func validateAccessCrossConfig(categories config.TicketCategoriesConfig, access config.AccessConfig) {
	staffAccess, ok := access["Tickets"]
	if !ok {
		return
	}
	entry, ok := staffAccess["Manage"]
	if !ok {
		return
	}
	allowed := make(map[string]struct{}, len(entry.Roles))
	for _, role := range entry.Roles {
		allowed[role] = struct{}{}
	}

	for key, category := range categories {
		for _, role := range category.Roles {
			if role == "*" || role == "Admin" {
				continue
			}
			if _, ok := allowed[role]; !ok {
				panic(fmt.Errorf("TicketCategories.%s lists role %q but Tickets.Manage does not; that role can never reach the category", key, role))
			}
		}
	}
}
