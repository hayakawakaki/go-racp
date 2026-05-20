package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/stall/domain"
)

type fakeRepo struct {
	err      error
	loadHook func(ctx context.Context, call int) ([]domain.Vendor, error)
	vendors  []domain.Vendor
	calls    int
	mu       sync.Mutex
}

func (f *fakeRepo) LoadAll(ctx context.Context) ([]domain.Vendor, error) {
	f.mu.Lock()
	f.calls++
	call := f.calls
	hook := f.loadHook
	v := f.vendors
	err := f.err
	f.mu.Unlock()
	if hook != nil {
		return hook(ctx, call)
	}

	return v, err
}

func (f *fakeRepo) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.calls
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestPoller_FirstTickPopulatesSnapshot(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{vendors: []domain.Vendor{
		{ID: 1, Type: domain.VendorTypeSelling, Items: []domain.VendorItem{{ItemID: 501, Amount: 1, Price: 100}}},
	}}
	p := NewPoller(repo, time.Hour, discardLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		p.Run(ctx)
		close(done)
	}()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if p.Snapshot() != nil {
			break
		}
		time.Sleep(time.Millisecond)
	}

	snap := p.Snapshot()
	if snap == nil {
		t.Fatalf("snapshot still nil after first tick")
	}
	if len(snap.Vendors) != 1 || snap.Vendors[0].ID != 1 {
		t.Errorf("snapshot vendors = %+v", snap.Vendors)
	}
	if _, ok := snap.ByKey[domain.VendorKey{Type: domain.VendorTypeSelling, ID: 1}]; !ok {
		t.Errorf("ByKey missing vendor key")
	}
	if keys := snap.ByItem[501]; len(keys) != 1 {
		t.Errorf("ByItem[501] = %+v, want one key", keys)
	}

	cancel()
	<-done
}

func TestPoller_ErrorKeepsPreviousSnapshot(t *testing.T) {
	t.Parallel()
	var (
		repo  fakeRepo
		first atomic.Bool
	)
	repo.loadHook = func(ctx context.Context, call int) ([]domain.Vendor, error) {
		if call == 1 {
			first.Store(true)
			return []domain.Vendor{{ID: 1, Type: domain.VendorTypeSelling}}, nil
		}
		return nil, errors.New("transient")
	}
	p := NewPoller(&repo, 5*time.Millisecond, discardLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		p.Run(ctx)
		close(done)
	}()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if first.Load() && repo.callCount() >= 3 {
			break
		}
		time.Sleep(time.Millisecond)
	}

	snap := p.Snapshot()
	if snap == nil {
		t.Fatalf("snapshot lost after errors")
	}
	if len(snap.Vendors) != 1 || snap.Vendors[0].ID != 1 {
		t.Errorf("snapshot replaced after error: %+v", snap.Vendors)
	}

	cancel()
	<-done
}

func TestPoller_ContextCancellationStopsRun(t *testing.T) {
	t.Parallel()
	p := NewPoller(&fakeRepo{}, time.Hour, discardLogger())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		p.Run(ctx)
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("Run did not exit after ctx cancel")
	}
}
