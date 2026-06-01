package metric

import (
	"context"
	"database/sql"
	"log/slog"
	"net"
	"strconv"
	"time"

	"github.com/hayakawakaki/go-racp/internal/platform/metric/app"
	"github.com/hayakawakaki/go-racp/internal/platform/metric/domain"
	"github.com/hayakawakaki/go-racp/internal/platform/metric/infra"
	"github.com/hayakawakaki/go-racp/server/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Reader = app.Reader

type Deps struct {
	MainDB *sql.DB
	Pool   *pgxpool.Pool
	Logger *slog.Logger
	Config *config.AppConfig
	Env    *config.EnvConfig
}

func Start(ctx context.Context, deps Deps) *Reader {
	source := infra.NewMariaSource(deps.MainDB)
	peakRepo := infra.NewPeakRepository(deps.Pool)
	generalRepo := infra.NewGeneralRepository(deps.Pool)

	windows := make([]domain.Window, 0, len(deps.Config.Metrics.PeakWindows))
	for _, w := range deps.Config.Metrics.PeakWindows {
		windows = append(windows, domain.Window(w))
	}

	online := app.NewOnlinePoller(app.OnlinePollerConfig{
		Source:   source,
		PeakSink: peakRepo,
		Bridge:   deps.MainDB,
		Logger:   deps.Logger,
		Location: deps.Config.General.Location(),
		Windows:  windows,
		Interval: deps.Config.Metrics.OnlinePollInterval,
		Gepard:   deps.Config.General.Gepard,
	})
	go online.Run(ctx)

	general := app.NewGeneralPoller(app.GeneralPollerConfig{
		Source:   source,
		Sink:     generalRepo,
		Bridge:   deps.MainDB,
		Logger:   deps.Logger,
		Interval: deps.Config.Metrics.GeneralPollInterval,
	})
	go general.Run(ctx)

	const dialTimeout = 5 * time.Second
	probe := infra.NewTCPProbe(dialTimeout)
	status := app.NewStatusPoller(app.StatusPollerConfig{
		Probe:        probe,
		Logger:       deps.Logger,
		LoginAddress: net.JoinHostPort(deps.Env.ServerHost, strconv.Itoa(deps.Env.LoginPort)),
		CharAddress:  net.JoinHostPort(deps.Env.ServerHost, strconv.Itoa(deps.Env.CharPort)),
		MapAddress:   net.JoinHostPort(deps.Env.ServerHost, strconv.Itoa(deps.Env.MapPort)),
		WebAddress:   net.JoinHostPort(deps.Env.ServerHost, strconv.Itoa(deps.Env.WebPort)),
		Interval:     deps.Config.Metrics.StatusPollInterval,
	})
	go status.Run(ctx)

	reader := app.NewReader(app.ReaderConfig{
		Online:   online,
		Status:   status,
		Peaks:    peakRepo,
		General:  generalRepo,
		Location: deps.Config.General.Location(),
		Windows:  windows,
	})
	SetLive(reader)

	return reader
}
