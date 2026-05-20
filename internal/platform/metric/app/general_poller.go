package app

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/hayakawakaki/go-racp/internal/platform/metric/domain"
	"golang.org/x/sync/errgroup"
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
	cfg.Logger = cmp.Or(cfg.Logger, slog.Default())
	return &GeneralPoller{cfg: cfg}
}

func (p *GeneralPoller) Run(ctx context.Context) {
	runTicker(ctx, p.cfg.Interval, p.RefreshOnce)
}

func (p *GeneralPoller) RefreshOnce(ctx context.Context) {
	var accounts, characters, guilds int
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		v, err := p.cfg.Source.CountAccounts(gctx)
		if err != nil {
			return fmt.Errorf("accounts: %w", err)
		}
		accounts = v
		return nil
	})
	g.Go(func() error {
		v, err := p.cfg.Source.CountCharacters(gctx)
		if err != nil {
			return fmt.Errorf("characters: %w", err)
		}
		characters = v
		return nil
	})
	g.Go(func() error {
		v, err := p.cfg.Source.CountGuilds(gctx)
		if err != nil {
			return fmt.Errorf("guilds: %w", err)
		}
		guilds = v
		return nil
	})
	if err := g.Wait(); err != nil {
		p.cfg.Logger.Warn("metric: general count query failed", "err", err)
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
