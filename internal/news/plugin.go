package news

import (
	"net/http"

	accountinfra "github.com/hayakawakaki/go-racp/internal/account/infra"
	platinfra "github.com/hayakawakaki/go-racp/internal/infra"
	newsapp "github.com/hayakawakaki/go-racp/internal/news/app"
	"github.com/hayakawakaki/go-racp/internal/news/domain"
	newsinfra "github.com/hayakawakaki/go-racp/internal/news/infra"
	newstransport "github.com/hayakawakaki/go-racp/internal/news/transport"
	"github.com/hayakawakaki/go-racp/internal/plugin"
	"github.com/hayakawakaki/go-racp/internal/routes"
	"github.com/hayakawakaki/go-racp/server/config"
)

func init() {
	plugin.Register(plugin.Plugin{Name: "news", Mount: mount})
}

func mount(reg *routes.Registry, mux *http.ServeMux, in *platinfra.Infra) {
	access := config.ProcessAccessConfig()
	categories := buildCategoryResolver(in.Config.App.NewsCategories)
	repo := newsinfra.NewRepository(in.DB)
	service := newsapp.NewService(repo, categories, in.Logger)
	renderer := newsinfra.NewRenderer()
	userRepo := accountinfra.NewRepository(in.MainDB)

	handler := newstransport.NewHandler(service, renderer, newstransport.HandlerConfig{
		Logger:      in.Logger,
		Users:       userRepo,
		Roles:       in.Roles,
		ManageRoles: manageRoles(access),
		General:     in.Config.App.General,
	})
	handler.RegisterRoutes(reg, mux)
}

func manageRoles(access config.AccessConfig) []string {
	news, ok := access["News"]
	if !ok {
		return nil
	}
	list, ok := news["Manage"]
	if !ok {
		return nil
	}

	return list
}

func buildCategoryResolver(cfg config.NewsCategoriesConfig) domain.CategoryResolver {
	list := make([]domain.Category, 0, len(cfg))
	for key, c := range cfg {
		list = append(list, domain.Category{Key: key, Display: c.Display})
	}

	return domain.NewCategoryResolver(list)
}
