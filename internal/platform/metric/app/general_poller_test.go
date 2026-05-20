package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/platform/metric/domain"
)

type fakeGeneralSource struct {
	accountsErr   error
	charactersErr error
	guildsErr     error
	accounts      int
	characters    int
	guilds        int
}

func (f *fakeGeneralSource) CountAccounts(context.Context) (int, error) {
	return f.accounts, f.accountsErr
}

func (f *fakeGeneralSource) CountCharacters(context.Context) (int, error) {
	return f.characters, f.charactersErr
}

func (f *fakeGeneralSource) CountGuilds(context.Context) (int, error) {
	return f.guilds, f.guildsErr
}

type fakeGeneralSink struct {
	err      error
	inserted []domain.GeneralSnapshot
	mu       sync.Mutex
}

func (f *fakeGeneralSink) Insert(_ context.Context, snap domain.GeneralSnapshot) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.inserted = append(f.inserted, snap)
	return nil
}

func (f *fakeGeneralSink) calls() []domain.GeneralSnapshot {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]domain.GeneralSnapshot, len(f.inserted))
	copy(out, f.inserted)
	return out
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestGeneralPoller_RefreshOnce_HappyPath(t *testing.T) {
	t.Parallel()
	src := &fakeGeneralSource{accounts: 10, characters: 50, guilds: 3}
	sink := &fakeGeneralSink{}
	p := NewGeneralPoller(GeneralPollerConfig{Source: src, Sink: sink, Logger: discardLogger(), Interval: time.Hour})

	p.RefreshOnce(context.Background())

	calls := sink.calls()
	if len(calls) != 1 {
		t.Fatalf("Insert calls = %d, want 1", len(calls))
	}
	got := calls[0]
	if got.TotalAccounts != 10 || got.TotalCharacters != 50 || got.TotalGuilds != 3 {
		t.Errorf("inserted = %+v, want {accounts:10, characters:50, guilds:3}", got)
	}
}

func TestGeneralPoller_RefreshOnce_AnySourceErrorSkipsInsert(t *testing.T) {
	t.Parallel()

	tests := []struct {
		src  *fakeGeneralSource
		name string
	}{
		{name: "accounts errors", src: &fakeGeneralSource{accountsErr: errors.New("boom")}},
		{name: "characters errors", src: &fakeGeneralSource{charactersErr: errors.New("boom")}},
		{name: "guilds errors", src: &fakeGeneralSource{guildsErr: errors.New("boom")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sink := &fakeGeneralSink{}
			p := NewGeneralPoller(GeneralPollerConfig{Source: tt.src, Sink: sink, Logger: discardLogger(), Interval: time.Hour})

			p.RefreshOnce(context.Background())

			if calls := sink.calls(); len(calls) != 0 {
				t.Errorf("Insert called despite source error: %+v", calls)
			}
		})
	}
}

func TestGeneralPoller_RefreshOnce_SinkErrorDoesNotPanic(t *testing.T) {
	t.Parallel()
	src := &fakeGeneralSource{accounts: 1, characters: 2, guilds: 3}
	sink := &fakeGeneralSink{err: errors.New("sink down")}
	p := NewGeneralPoller(GeneralPollerConfig{Source: src, Sink: sink, Logger: discardLogger(), Interval: time.Hour})

	p.RefreshOnce(context.Background())
}

func TestGeneralPoller_NewGeneralPoller_DefaultsLogger(t *testing.T) {
	t.Parallel()
	p := NewGeneralPoller(GeneralPollerConfig{Source: &fakeGeneralSource{}, Sink: &fakeGeneralSink{}})
	if p.cfg.Logger == nil {
		t.Errorf("Logger not defaulted")
	}
}

func TestGeneralPoller_Run_ExitsOnContextCancel(t *testing.T) {
	t.Parallel()
	p := NewGeneralPoller(GeneralPollerConfig{
		Source: &fakeGeneralSource{}, Sink: &fakeGeneralSink{}, Logger: discardLogger(), Interval: time.Hour,
	})

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
