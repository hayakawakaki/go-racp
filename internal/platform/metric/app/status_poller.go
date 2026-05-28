package app

import (
	"cmp"
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hayakawakaki/go-racp/internal/platform/metric/domain"
)

type PortProbe interface {
	Probe(ctx context.Context, address string) (bool, error)
}

type StatusPollerConfig struct {
	Probe        PortProbe
	Logger       *slog.Logger
	Now          func() time.Time
	LoginAddress string
	CharAddress  string
	MapAddress   string
	WebAddress   string
	Interval     time.Duration
}

type StatusPoller struct {
	snapshot atomic.Pointer[domain.ServerStatusSnapshot]
	cfg      StatusPollerConfig
}

func NewStatusPoller(cfg StatusPollerConfig) *StatusPoller {
	cfg.Logger = cmp.Or(cfg.Logger, slog.Default())
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &StatusPoller{cfg: cfg}
}

func (p *StatusPoller) Snapshot() domain.ServerStatusSnapshot {
	snap := p.snapshot.Load()
	if snap == nil {
		return domain.ServerStatusSnapshot{}
	}
	return *snap
}

func (p *StatusPoller) Run(ctx context.Context) {
	runTicker(ctx, p.cfg.Interval, p.RefreshOnce)
}

func (p *StatusPoller) RefreshOnce(ctx context.Context) {
	previous := p.snapshot.Load()
	firstRun := previous == nil

	results := []struct {
		err     error
		name    string
		address string
		up      bool
		wasUp   bool
	}{
		{name: "login", address: p.cfg.LoginAddress},
		{name: "char", address: p.cfg.CharAddress},
		{name: "map", address: p.cfg.MapAddress},
		{name: "web", address: p.cfg.WebAddress},
	}
	if !firstRun {
		results[0].wasUp = previous.Login
		results[1].wasUp = previous.Char
		results[2].wasUp = previous.Map
		results[3].wasUp = previous.Web
	}

	var waitGroup sync.WaitGroup
	for index := range results {
		waitGroup.Go(func() {
			results[index].up, results[index].err = p.cfg.Probe.Probe(ctx, results[index].address)
		})
	}
	waitGroup.Wait()

	if ctx.Err() == nil {
		for _, result := range results {
			p.logTransition(firstRun, result.wasUp, result.up, result.name, result.address, result.err)
		}
	}

	snap := domain.ServerStatusSnapshot{
		CheckedAt: p.cfg.Now(),
		Login:     results[0].up,
		Char:      results[1].up,
		Map:       results[2].up,
		Web:       results[3].up,
	}

	p.snapshot.Store(&snap)
}

func (p *StatusPoller) logTransition(firstRun, wasUp, up bool, name, address string, probeErr error) {
	switch {
	case !up && (firstRun || wasUp):
		p.cfg.Logger.Warn("metric: server down", "service", name, "address", address, "err", probeErr)
	case up && !firstRun && !wasUp:
		p.cfg.Logger.Info("metric: server recovered", "service", name, "address", address)
	}
}
