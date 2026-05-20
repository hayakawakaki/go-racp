package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/hayakawakaki/go-racp/internal/platform/metric/domain"
)

type GeneralSource interface {
	CountAccounts(ctx context.Context) (int, error)
	CountCharacters(ctx context.Context) (int, error)
	CountGuilds(ctx context.Context) (int, error)
}

type GeneralSink interface {
	Insert(ctx context.Context, snap domain.GeneralSnapshot) error
}

type GeneralPollerConfig struct {
	Source   GeneralSource
	Sink     GeneralSink
	Logger   *slog.Logger
	Interval time.Duration
}

type GeneralPoller struct {
	cfg GeneralPollerConfig
}

func NewGeneralPoller(cfg GeneralPollerConfig) *GeneralPoller {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &GeneralPoller{cfg: cfg}
}

func (p *GeneralPoller) Run(ctx context.Context) {
	p.RefreshOnce(ctx)
	ticker := time.NewTicker(p.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.RefreshOnce(ctx)
		}
	}
}

func (p *GeneralPoller) RefreshOnce(ctx context.Context) {
	accounts, err := p.cfg.Source.CountAccounts(ctx)
	if err != nil {
		p.cfg.Logger.Warn("metric: account count query failed", "err", err)
		return
	}
	characters, err := p.cfg.Source.CountCharacters(ctx)
	if err != nil {
		p.cfg.Logger.Warn("metric: character count query failed", "err", err)
		return
	}
	guilds, err := p.cfg.Source.CountGuilds(ctx)
	if err != nil {
		p.cfg.Logger.Warn("metric: guild count query failed", "err", err)
		return
	}

	if err := p.cfg.Sink.Insert(ctx, domain.GeneralSnapshot{
		TotalAccounts:   accounts,
		TotalCharacters: characters,
		TotalGuilds:     guilds,
	}); err != nil {
		p.cfg.Logger.Warn("metric: general snapshot insert failed", "err", err)
	}
}
