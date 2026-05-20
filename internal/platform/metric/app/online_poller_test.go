package app

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/platform/metric/domain"
)

type fakeOnlineSource struct {
	totalErr  error
	vendorErr error
	uniqueErr error
	total     int
	vendor    int
	unique    int
}

func (f *fakeOnlineSource) CountOnlineTotal(context.Context) (int, error) {
	return f.total, f.totalErr
}

func (f *fakeOnlineSource) CountVendors(context.Context) (int, error) {
	return f.vendor, f.vendorErr
}

func (f *fakeOnlineSource) CountUniqueOnline(context.Context) (int, error) {
	return f.unique, f.uniqueErr
}

type peakCall struct {
	key    time.Time
	metric domain.MetricName
	window domain.Window
	value  int
}

type fakePeakSink struct {
	err   error
	calls []peakCall
	mu    sync.Mutex
}

func (f *fakePeakSink) UpsertIfGreater(_ context.Context, m domain.MetricName, w domain.Window, k time.Time, v int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, peakCall{metric: m, window: w, key: k, value: v})
	return f.err
}

func (f *fakePeakSink) snapshot() []peakCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]peakCall, len(f.calls))
	copy(out, f.calls)
	return out
}

func fixedNow(now time.Time) func() time.Time {
	return func() time.Time { return now }
}

func TestOnlinePoller_Snapshot_ZeroBeforeRefresh(t *testing.T) {
	t.Parallel()
	p := NewOnlinePoller(OnlinePollerConfig{})

	snap := p.Snapshot()
	if snap != (domain.OnlineSnapshot{}) {
		t.Errorf("Snapshot before refresh = %+v, want zero value", snap)
	}
}

func TestOnlinePoller_RefreshOnce_StoresSnapshotAndDerivedFields(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	src := &fakeOnlineSource{total: 100, vendor: 30, unique: 80}
	sink := &fakePeakSink{}
	p := NewOnlinePoller(OnlinePollerConfig{
		Source:   src,
		PeakSink: sink,
		Logger:   discardLogger(),
		Now:      fixedNow(now),
		Location: time.UTC,
		Windows:  []domain.Window{domain.WindowDaily},
		Gepard:   false,
	})

	p.RefreshOnce(context.Background())

	snap := p.Snapshot()
	if snap.Total != 100 || snap.Vendor != 30 || snap.NonVendor != 70 {
		t.Errorf("snapshot counts = %+v, want total=100 vendor=30 non_vendor=70", snap)
	}
	if snap.Unique != 0 || snap.HasUnique {
		t.Errorf("unique fields = (%d, has=%v), want zero when gepard=false", snap.Unique, snap.HasUnique)
	}
	if !snap.UpdatedAt.Equal(now) {
		t.Errorf("UpdatedAt = %v, want %v", snap.UpdatedAt, now)
	}
}

func TestOnlinePoller_RefreshOnce_GepardTracksUnique(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	src := &fakeOnlineSource{total: 100, vendor: 30, unique: 80}
	sink := &fakePeakSink{}
	p := NewOnlinePoller(OnlinePollerConfig{
		Source:   src,
		PeakSink: sink,
		Logger:   discardLogger(),
		Now:      fixedNow(now),
		Location: time.UTC,
		Windows:  []domain.Window{domain.WindowDaily},
		Gepard:   true,
	})

	p.RefreshOnce(context.Background())

	snap := p.Snapshot()
	if snap.Unique != 80 || !snap.HasUnique {
		t.Errorf("unique fields = (%d, has=%v), want (80, true)", snap.Unique, snap.HasUnique)
	}
}

func TestOnlinePoller_RefreshOnce_PeakUpsertsAllMetricsTimesWindows(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	src := &fakeOnlineSource{total: 100, vendor: 30, unique: 80}
	sink := &fakePeakSink{}
	windows := []domain.Window{domain.WindowDaily, domain.WindowAllTime}
	p := NewOnlinePoller(OnlinePollerConfig{
		Source: src, PeakSink: sink, Logger: discardLogger(),
		Now: fixedNow(now), Location: time.UTC, Windows: windows, Gepard: false,
	})

	p.RefreshOnce(context.Background())

	got := sink.snapshot()
	if len(got) != 6 {
		t.Fatalf("UpsertIfGreater calls = %d, want 6 (3 metrics * 2 windows)", len(got))
	}

	wantValues := map[domain.MetricName]int{
		domain.MetricOnlineTotal:     100,
		domain.MetricOnlineVendor:    30,
		domain.MetricOnlineNonVendor: 70,
	}
	seen := map[domain.MetricName]map[domain.Window]bool{}
	for _, c := range got {
		if c.metric == domain.MetricOnlineUnique {
			t.Errorf("unique upserted despite gepard=false: %+v", c)
		}
		if v, ok := wantValues[c.metric]; ok && v != c.value {
			t.Errorf("metric %v value = %d, want %d", c.metric, c.value, v)
		}
		if seen[c.metric] == nil {
			seen[c.metric] = map[domain.Window]bool{}
		}
		seen[c.metric][c.window] = true
	}
	for metric := range wantValues {
		for _, w := range windows {
			if !seen[metric][w] {
				t.Errorf("missing upsert for %v/%v", metric, w)
			}
		}
	}
}

func TestOnlinePoller_RefreshOnce_GepardAddsUniqueUpserts(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	src := &fakeOnlineSource{total: 100, vendor: 30, unique: 80}
	sink := &fakePeakSink{}
	p := NewOnlinePoller(OnlinePollerConfig{
		Source: src, PeakSink: sink, Logger: discardLogger(),
		Now: fixedNow(now), Location: time.UTC,
		Windows: []domain.Window{domain.WindowDaily}, Gepard: true,
	})

	p.RefreshOnce(context.Background())

	got := sink.snapshot()
	if len(got) != 4 {
		t.Fatalf("UpsertIfGreater calls = %d, want 4 (4 metrics * 1 window)", len(got))
	}
	var uniqueValue int
	for _, c := range got {
		if c.metric == domain.MetricOnlineUnique {
			uniqueValue = c.value
		}
	}
	if uniqueValue != 80 {
		t.Errorf("unique upsert value = %d, want 80", uniqueValue)
	}
}

func TestOnlinePoller_RefreshOnce_SourceErrorKeepsPreviousSnapshot(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	src := &fakeOnlineSource{total: 100, vendor: 30}
	sink := &fakePeakSink{}
	p := NewOnlinePoller(OnlinePollerConfig{
		Source: src, PeakSink: sink, Logger: discardLogger(),
		Now: fixedNow(now), Location: time.UTC,
		Windows: []domain.Window{domain.WindowDaily}, Gepard: false,
	})
	p.RefreshOnce(context.Background())

	priorPeakCalls := len(sink.snapshot())
	src.totalErr = errors.New("boom")

	p.RefreshOnce(context.Background())

	snap := p.Snapshot()
	if snap.Total != 100 || snap.Vendor != 30 {
		t.Errorf("snapshot replaced after source error: %+v", snap)
	}
	if got := len(sink.snapshot()); got != priorPeakCalls {
		t.Errorf("peak upserts after error = %d, want unchanged %d", got, priorPeakCalls)
	}
}

func TestOnlinePoller_RefreshOnce_PeakKeyComputedInConfigLocation(t *testing.T) {
	t.Parallel()
	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Skipf("Asia/Tokyo unavailable: %v", err)
	}
	utcMidnight := time.Date(2026, 5, 20, 15, 30, 0, 0, time.UTC)
	src := &fakeOnlineSource{total: 1, vendor: 0}
	sink := &fakePeakSink{}
	p := NewOnlinePoller(OnlinePollerConfig{
		Source: src, PeakSink: sink, Logger: discardLogger(),
		Now: fixedNow(utcMidnight), Location: tokyo,
		Windows: []domain.Window{domain.WindowDaily}, Gepard: false,
	})

	p.RefreshOnce(context.Background())

	calls := sink.snapshot()
	if len(calls) == 0 {
		t.Fatalf("no peak calls")
	}
	wantKey := time.Date(2026, 5, 21, 0, 0, 0, 0, tokyo)
	if !calls[0].key.Equal(wantKey) {
		t.Errorf("daily key = %v, want %v (tokyo midnight)", calls[0].key, wantKey)
	}
}

func TestOnlinePoller_NewOnlinePoller_Defaults(t *testing.T) {
	t.Parallel()
	p := NewOnlinePoller(OnlinePollerConfig{})
	if p.cfg.Logger == nil {
		t.Errorf("Logger not defaulted")
	}
	if p.cfg.Now == nil {
		t.Errorf("Now not defaulted")
	}
	if p.cfg.Location == nil {
		t.Errorf("Location not defaulted")
	}
}
