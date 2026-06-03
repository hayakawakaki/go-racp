package notification

import (
	"log/slog"

	"github.com/hayakawakaki/go-racp/internal/platform/notification/app"
	"github.com/hayakawakaki/go-racp/internal/platform/notification/infra"
	"github.com/hayakawakaki/go-racp/server/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Start(pool *pgxpool.Pool, cfg config.NotificationConfig, logger *slog.Logger) *app.Service {
	repo := infra.NewRepository(pool)
	broadcaster := app.NewBroadcaster()

	return app.NewService(repo, broadcaster, logger,
		app.WithRecentLimit(cfg.RecentLimit),
		app.WithRetention(cfg.Retention),
	)
}
