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
	Probe(ctx context.Context, address string) bool
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
	var login, char, mapServer, web bool

	probes := []struct {
		result  *bool
		address string
	}{
		{&login, p.cfg.LoginAddress},
		{&char, p.cfg.CharAddress},
		{&mapServer, p.cfg.MapAddress},
		{&web, p.cfg.WebAddress},
	}

	var waitGroup sync.WaitGroup
	for _, probe := range probes {
		waitGroup.Go(func() {
			*probe.result = p.cfg.Probe.Probe(ctx, probe.address)
		})
	}
	waitGroup.Wait()

	snap := domain.ServerStatusSnapshot{
		CheckedAt: p.cfg.Now(),
		Login:     login,
		Char:      char,
		Map:       mapServer,
		Web:       web,
	}

	p.snapshot.Store(&snap)
}
