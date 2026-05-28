package news

import (
	"net/http"
	"sort"

	accinfra "github.com/hayakawakaki/go-racp/internal/features/account/infra"
	"github.com/hayakawakaki/go-racp/internal/features/news/app"
	"github.com/hayakawakaki/go-racp/internal/features/news/domain"
	"github.com/hayakawakaki/go-racp/internal/features/news/infra"
	"github.com/hayakawakaki/go-racp/internal/features/news/transport"
	coreinfra "github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/platform/plugin"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
	"github.com/hayakawakaki/go-racp/internal/platform/theme"
	"github.com/hayakawakaki/go-racp/server/config"
)

func init() {
	plugin.Register(plugin.Plugin{Name: "news", Mount: mount})
}

func mount(reg *routes.Registry, mux *http.ServeMux, in *coreinfra.Infra) {
	access := config.ProcessAccessConfig(theme.ActiveAccessYAML)
	categories := buildCategoryResolver(in.Config.App.NewsCategories)
	repo := infra.NewRepository(in.DB)
	service := app.NewService(repo, categories, in.Logger)
	app.SetLive(service)
	renderer := infra.NewRenderer(in.Logger)
	userRepo := accinfra.NewRepository(in.MainDB)

	handler := transport.NewHandler(service, renderer, transport.HandlerConfig{
		Logger:      in.Logger,
		Users:       userRepo,
		Roles:       in.Roles,
		ManageRoles: access.ManageRoles("News"),
		General:     in.Config.App.General,
		Theme:       theme.Active,
	})
	handler.RegisterRoutes(reg, mux)
}

func buildCategoryResolver(cfg config.NewsCategoriesConfig) domain.CategoryResolver {
	keys := make([]string, 0, len(cfg))
	for key := range cfg {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	list := make([]domain.Category, 0, len(cfg))
	for _, key := range keys {
		list = append(list, domain.Category{Key: key, Display: cfg[key].Display})
	}

	return domain.NewCategoryResolver(list)
}
