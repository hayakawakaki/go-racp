package admin

import (
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/account/app"
	accountinfra "github.com/hayakawakaki/go-racp/internal/account/infra"
	"github.com/hayakawakaki/go-racp/internal/account/transport/middleware"
	"github.com/hayakawakaki/go-racp/internal/admin/transport"
	"github.com/hayakawakaki/go-racp/internal/httpx"
	platinfra "github.com/hayakawakaki/go-racp/internal/infra"
	"github.com/hayakawakaki/go-racp/internal/plugin"
)

func init() {
	plugin.Register(plugin.Plugin{
		Name:  "admin",
		Mount: mount,
	})
}

func mount(mux *http.ServeMux, in *platinfra.Infra) {
	userRepo := accountinfra.NewRepository(in.MainDB)
	sessRepo := accountinfra.NewSessionRepository(in.MainDB)
	sessSvc := app.NewSessionService(sessRepo, in.Config.App.TTL.Session)

	secure := in.Config.Env.Mode != "development"
	layout := httpx.Layout{GeneralConfig: in.Config.App.General}
	requireAdmin := middleware.RequireRoleHidden(sessSvc, userRepo, in.Roles, in.Logger, secure, layout)

	h := transport.NewHandler(transport.HandlerConfig{
		Logger:  in.Logger,
		General: in.Config.App.General,
	})
	h.RegisterRoutes(mux, requireAdmin)
}
