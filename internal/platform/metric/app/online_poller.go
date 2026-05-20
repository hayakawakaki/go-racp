package app

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/hayakawakaki/go-racp/internal/platform/metric/domain"
)

type OnlineSource interface {
	CountOnlineTotal(ctx context.Context) (int, error)
	CountVendors(ctx context.Context) (int, error)
	CountUniqueOnline(ctx context.Context) (int, error)
}

type PeakSink interface {
	UpsertIfGreater(ctx context.Context, m domain.MetricName, w domain.Window, k time.Time, v int) error
}

type OnlinePollerConfig struct {
	Source   OnlineSource
	PeakSink PeakSink
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
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Location == nil {
		cfg.Location = time.UTC
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

func (p *OnlinePoller) RefreshOnce(ctx context.Context) {
	total, err := p.cfg.Source.CountOnlineTotal(ctx)
	if err != nil {
		p.cfg.Logger.Warn("metric: online_total query failed", "err", err)
		return
	}
	vendor, err := p.cfg.Source.CountVendors(ctx)
	if err != nil {
		p.cfg.Logger.Warn("metric: vendor count query failed", "err", err)
		return
	}

	snap := domain.OnlineSnapshot{
		UpdatedAt: p.cfg.Now(),
		Total:     total,
		Vendor:    vendor,
		NonVendor: total - vendor,
	}

	if p.cfg.Gepard {
		unique, err := p.cfg.Source.CountUniqueOnline(ctx)
		if err != nil {
			p.cfg.Logger.Warn("metric: unique online query failed", "err", err)
			return
		}
		snap.Unique = unique
		snap.HasUnique = true
	}

	p.snapshot.Store(&snap)
	p.upsertPeaks(ctx, snap)
}

func (p *OnlinePoller) upsertPeaks(ctx context.Context, snap domain.OnlineSnapshot) {
	nowInTZ := p.cfg.Now().In(p.cfg.Location)
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
