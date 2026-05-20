package app

import (
	"context"
	"log/slog"
	"sort"
	"sync/atomic"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/stall/domain"
)

type Poller struct {
	repo     domain.Repository
	snapshot atomic.Pointer[domain.Snapshot]
	logger   *slog.Logger
	interval time.Duration
}

func NewPoller(repo domain.Repository, interval time.Duration, logger *slog.Logger) *Poller {
	log := logger
	if log == nil {
		log = slog.Default()
	}

	return &Poller{repo: repo, interval: interval, logger: log}
}

func (p *Poller) Snapshot() *domain.Snapshot {
	return p.snapshot.Load()
}

func (p *Poller) Run(ctx context.Context) {
	p.refreshOnce(ctx)

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.refreshOnce(ctx)
		}
	}
}

func (p *Poller) refreshOnce(ctx context.Context) {
	vendors, err := p.repo.LoadAll(ctx)
	if err != nil {
		p.logger.Warn("stall: poll failed, retaining previous snapshot", "err", err)
		return
	}
	p.snapshot.Store(buildSnapshot(vendors))
}

func buildSnapshot(vendors []domain.Vendor) *domain.Snapshot {
	sort.Slice(vendors, func(i, j int) bool { return vendors[i].ID > vendors[j].ID })

	byKey := make(map[domain.VendorKey]*domain.Vendor, len(vendors))
	byItem := map[int][]domain.VendorKey{}
	for index := range vendors {
		v := &vendors[index]
		key := v.Key()
		byKey[key] = v
		for _, item := range v.Items {
			byItem[item.ItemID] = append(byItem[item.ItemID], key)
		}
	}

	return &domain.Snapshot{
		LoadedAt: time.Now(),
		Vendors:  vendors,
		ByKey:    byKey,
		ByItem:   byItem,
	}
}
