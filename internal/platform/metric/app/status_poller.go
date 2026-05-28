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
	results := []struct {
		err     error
		name    string
		address string
		up      bool
	}{
		{name: "login", address: p.cfg.LoginAddress},
		{name: "char", address: p.cfg.CharAddress},
		{name: "map", address: p.cfg.MapAddress},
		{name: "web", address: p.cfg.WebAddress},
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
			if result.err != nil {
				p.cfg.Logger.Warn("metric: server probe failed", "service", result.name, "address", result.address, "err", result.err)
			}
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
