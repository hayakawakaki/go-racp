package app

import (
	"context"
	"fmt"
	"time"

	"github.com/hayakawakaki/go-racp/internal/platform/metric/domain"
)

type PeakReader interface {
	Current(ctx context.Context, keys map[domain.Window]time.Time) ([]domain.PeakRow, error)
}

type GeneralReader interface {
	Latest(ctx context.Context) (domain.GeneralSnapshot, error)
}

type Reader struct {
	online   *OnlinePoller
	peaks    PeakReader
	general  GeneralReader
	location *time.Location
	now      func() time.Time
	windows  []domain.Window
}

type ReaderConfig struct {
	Online   *OnlinePoller
	Peaks    PeakReader
	General  GeneralReader
	Location *time.Location
	Now      func() time.Time
	Windows  []domain.Window
}

func NewReader(cfg ReaderConfig) *Reader {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Reader{
		online:   cfg.Online,
		peaks:    cfg.Peaks,
		general:  cfg.General,
		windows:  cfg.Windows,
		location: cfg.Location,
		now:      cfg.Now,
	}
}

func (r *Reader) Online(_ context.Context) domain.OnlineSnapshot {
	return r.online.Snapshot()
}

func (r *Reader) Peaks(ctx context.Context) ([]domain.PeakRow, error) {
	if len(r.windows) == 0 {
		return nil, nil
	}
	keys := make(map[domain.Window]time.Time, len(r.windows))
	nowInTZ := r.now().In(r.location)
	for _, w := range r.windows {
		keys[w] = domain.WindowKey(w, nowInTZ)
	}
	rows, err := r.peaks.Current(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("app.Reader.Peaks: %w", err)
	}
	return rows, nil
}

func (r *Reader) General(ctx context.Context) (domain.GeneralSnapshot, error) {
	snap, err := r.general.Latest(ctx)
	if err != nil {
		return domain.GeneralSnapshot{}, fmt.Errorf("app.Reader.General: %w", err)
	}
	return snap, nil
}

func (r *Reader) AllTimePeakOnline(ctx context.Context) (int, error) {
	rows, err := r.Peaks(ctx)
	if err != nil {
		return 0, err
	}
	return domain.NewPeakSet(rows).Value(domain.WindowAllTime, domain.MetricOnlineTotal), nil
}
