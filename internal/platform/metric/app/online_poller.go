package app

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/hayakawakaki/go-racp/internal/platform/metric/domain"
	"golang.org/x/sync/errgroup"
)

const bridgePingTimeout = 2 * time.Second

type OnlineSource interface {
	CountOnlineTotal(ctx context.Context) (int, error)
	CountVendors(ctx context.Context) (int, error)
	CountUniqueOnline(ctx context.Context) (int, error)
}

type BridgePinger interface {
	PingContext(ctx context.Context) error
}

func bridgeReachable(ctx context.Context, bridge BridgePinger) bool {
	pingCtx, cancel := context.WithTimeout(ctx, bridgePingTimeout)
	defer cancel()

	return bridge.PingContext(pingCtx) == nil
}

type PeakSink interface {
	UpsertIfGreater(ctx context.Context, m domain.MetricName, w domain.Window, k time.Time, v int) error
}

type OnlinePollerConfig struct {
	Source   OnlineSource
	PeakSink PeakSink
	Bridge   BridgePinger
	Logger   *slog.Logger
	Now      func() time.Time
	Location *time.Location
	Windows  []domain.Window
	Interval time.Duration
	Gepard   bool
}

type OnlinePoller struct {
	snapshot atomic.Pointer[domain.OnlineSnapshot]
	cfg      OnlinePollerConfig
}

func NewOnlinePoller(cfg OnlinePollerConfig) *OnlinePoller {
	cfg.Logger = cmp.Or(cfg.Logger, slog.Default())
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &OnlinePoller{cfg: cfg}
}

func (p *OnlinePoller) Snapshot() domain.OnlineSnapshot {
	snap := p.snapshot.Load()
	if snap == nil {
		return domain.OnlineSnapshot{}
	}
	return *snap
}

func (p *OnlinePoller) Run(ctx context.Context) {
	runTicker(ctx, p.cfg.Interval, p.RefreshOnce)
}

func (p *OnlinePoller) RefreshOnce(ctx context.Context) {
	if p.cfg.Bridge != nil && !bridgeReachable(ctx, p.cfg.Bridge) {
		return
	}

	var total, vendor, unique int
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		v, err := p.cfg.Source.CountOnlineTotal(gctx)
		if err != nil {
			return fmt.Errorf("online_total: %w", err)
		}
		total = v
		return nil
	})
	g.Go(func() error {
		v, err := p.cfg.Source.CountVendors(gctx)
		if err != nil {
			return fmt.Errorf("online_vendor: %w", err)
		}
		vendor = v
		return nil
	})
	if p.cfg.Gepard {
		g.Go(func() error {
			v, err := p.cfg.Source.CountUniqueOnline(gctx)
			if err != nil {
				return fmt.Errorf("online_unique: %w", err)
			}
			unique = v
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		p.cfg.Logger.Warn("metric: online count query failed", "err", err)
		return
	}

	snap := domain.OnlineSnapshot{
		UpdatedAt: p.cfg.Now(),
		Total:     total,
		Vendor:    vendor,
		NonVendor: total - vendor,
		Unique:    unique,
		HasUnique: p.cfg.Gepard,
	}

	p.snapshot.Store(&snap)
	p.upsertPeaks(ctx, snap)
}

func (p *OnlinePoller) upsertPeaks(ctx context.Context, snap domain.OnlineSnapshot) {
	nowInTZ := snap.UpdatedAt.In(p.cfg.Location)
	peaks := []struct {
		Metric domain.MetricName
		Value  int
		Track  bool
	}{
		{domain.MetricOnlineTotal, snap.Total, true},
		{domain.MetricOnlineNonVendor, snap.NonVendor, true},
		{domain.MetricOnlineVendor, snap.Vendor, true},
		{domain.MetricOnlineUnique, snap.Unique, snap.HasUnique},
	}

	for _, peak := range peaks {
		if !peak.Track {
			continue
		}
		for _, w := range p.cfg.Windows {
			key := domain.WindowKey(w, nowInTZ)
			if err := p.cfg.PeakSink.UpsertIfGreater(ctx, peak.Metric, w, key, peak.Value); err != nil {
				p.cfg.Logger.Warn("metric: peak upsert failed",
					"metric", peak.Metric, "window", w, "err", err)
			}
		}
	}
}
