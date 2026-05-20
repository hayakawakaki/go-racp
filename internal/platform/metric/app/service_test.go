package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/platform/metric/domain"
)

type fakePeakReader struct {
	err     error
	lastReq map[domain.Window]time.Time
	rows    []domain.PeakRow
	calls   int
}

func (f *fakePeakReader) Current(_ context.Context, keys map[domain.Window]time.Time) ([]domain.PeakRow, error) {
	f.calls++
	f.lastReq = keys
	return f.rows, f.err
}

type fakeGeneralReader struct {
	err  error
	snap domain.GeneralSnapshot
}

func (f *fakeGeneralReader) Latest(context.Context) (domain.GeneralSnapshot, error) {
	return f.snap, f.err
}

func newReaderWithSnap(snap domain.OnlineSnapshot, peaks *fakePeakReader, general *fakeGeneralReader, windows []domain.Window, now time.Time, loc *time.Location) *Reader {
	online := NewOnlinePoller(OnlinePollerConfig{})
	online.snapshot.Store(&snap)
	return NewReader(ReaderConfig{
		Online:   online,
		Peaks:    peaks,
		General:  general,
		Windows:  windows,
		Now:      func() time.Time { return now },
		Location: loc,
	})
}

func TestReader_Online_ReturnsCurrentSnapshot(t *testing.T) {
	t.Parallel()
	want := domain.OnlineSnapshot{Total: 42, Vendor: 5, NonVendor: 37}
	r := newReaderWithSnap(want, &fakePeakReader{}, &fakeGeneralReader{}, nil, time.Now(), time.UTC)

	got := r.Online(context.Background())
	if got != want {
		t.Errorf("Online = %+v, want %+v", got, want)
	}
}

func TestReader_Peaks_EmptyWindowsReturnsNil(t *testing.T) {
	t.Parallel()
	peaks := &fakePeakReader{}
	r := newReaderWithSnap(domain.OnlineSnapshot{}, peaks, &fakeGeneralReader{}, nil, time.Now(), time.UTC)

	rows, err := r.Peaks(context.Background())
	if err != nil {
		t.Fatalf("Peaks: %v", err)
	}
	if rows != nil {
		t.Errorf("rows = %v, want nil for empty windows", rows)
	}
	if peaks.calls != 0 {
		t.Errorf("Peaks.Current called despite empty windows")
	}
}

func TestReader_Peaks_BuildsKeysFromWindowsAndPassesThrough(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	peaks := &fakePeakReader{rows: []domain.PeakRow{
		{Metric: domain.MetricOnlineTotal, Window: domain.WindowDaily, Value: 100},
	}}
	windows := []domain.Window{domain.WindowDaily, domain.WindowMonthly}
	r := newReaderWithSnap(domain.OnlineSnapshot{}, peaks, &fakeGeneralReader{}, windows, now, time.UTC)

	rows, err := r.Peaks(context.Background())
	if err != nil {
		t.Fatalf("Peaks: %v", err)
	}
	if len(rows) != 1 || rows[0].Value != 100 {
		t.Errorf("rows = %+v", rows)
	}

	if got := peaks.lastReq[domain.WindowDaily]; !got.Equal(time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("daily key = %v", got)
	}
	if got := peaks.lastReq[domain.WindowMonthly]; !got.Equal(time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("monthly key = %v", got)
	}
}

func TestReader_Peaks_WrapsError(t *testing.T) {
	t.Parallel()
	peaks := &fakePeakReader{err: errors.New("query boom")}
	r := newReaderWithSnap(domain.OnlineSnapshot{}, peaks, &fakeGeneralReader{},
		[]domain.Window{domain.WindowDaily}, time.Now(), time.UTC)

	_, err := r.Peaks(context.Background())
	if err == nil || !strings.Contains(err.Error(), "app.Reader.Peaks") {
		t.Errorf("not wrapped: %v", err)
	}
}

func TestReader_General_ReturnsLatest(t *testing.T) {
	t.Parallel()
	want := domain.GeneralSnapshot{
		TotalAccounts: 10, TotalCharacters: 50, TotalGuilds: 3,
		CapturedAt: time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC),
	}
	r := newReaderWithSnap(domain.OnlineSnapshot{}, &fakePeakReader{},
		&fakeGeneralReader{snap: want}, nil, time.Now(), time.UTC)

	got, err := r.General(context.Background())
	if err != nil {
		t.Fatalf("General: %v", err)
	}
	if got != want {
		t.Errorf("General = %+v, want %+v", got, want)
	}
}

func TestReader_General_WrapsError(t *testing.T) {
	t.Parallel()
	r := newReaderWithSnap(domain.OnlineSnapshot{}, &fakePeakReader{},
		&fakeGeneralReader{err: errors.New("db down")}, nil, time.Now(), time.UTC)

	_, err := r.General(context.Background())
	if err == nil || !strings.Contains(err.Error(), "app.Reader.General") {
		t.Errorf("not wrapped: %v", err)
	}
}

func TestNewReader_DefaultsNowAndLocation(t *testing.T) {
	t.Parallel()
	r := NewReader(ReaderConfig{})
	if r.now == nil {
		t.Errorf("now not defaulted")
	}
	if r.location == nil {
		t.Errorf("location not defaulted")
	}
}
