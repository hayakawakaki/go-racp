package app

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/platform/metric/domain"
)

type probeOutcome struct {
	err error
	up  bool
}

type fakePortProbe struct {
	results map[string]probeOutcome
	probed  []string
	mu      sync.Mutex
}

func (f *fakePortProbe) Probe(_ context.Context, address string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.probed = append(f.probed, address)
	outcome := f.results[address]
	return outcome.up, outcome.err
}

func (f *fakePortProbe) probedAddresses() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.probed))
	copy(out, f.probed)
	return out
}

func captureLogger() (*slog.Logger, *bytes.Buffer) {
	buffer := &bytes.Buffer{}
	return slog.New(slog.NewTextHandler(buffer, nil)), buffer
}

func TestStatusPoller_Snapshot_ZeroBeforeRefresh(t *testing.T) {
	t.Parallel()
	p := NewStatusPoller(StatusPollerConfig{})

	if snap := p.Snapshot(); snap != (domain.ServerStatusSnapshot{}) {
		t.Errorf("Snapshot before refresh = %+v, want zero value", snap)
	}
}

func TestStatusPoller_RefreshOnce_MapsProbeResultsToServices(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	probe := &fakePortProbe{results: map[string]probeOutcome{
		"login-addr": {up: true},
		"char-addr":  {up: false},
		"map-addr":   {up: true},
		"web-addr":   {up: false},
	}}
	p := NewStatusPoller(StatusPollerConfig{
		Probe:        probe,
		Logger:       discardLogger(),
		Now:          fixedNow(now),
		LoginAddress: "login-addr",
		CharAddress:  "char-addr",
		MapAddress:   "map-addr",
		WebAddress:   "web-addr",
	})

	p.RefreshOnce(context.Background())

	snap := p.Snapshot()
	if !snap.Login || snap.Char || !snap.Map || snap.Web {
		t.Errorf("snapshot flags = %+v, want login=true char=false map=true web=false", snap)
	}
	if !snap.CheckedAt.Equal(now) {
		t.Errorf("CheckedAt = %v, want %v", snap.CheckedAt, now)
	}

	probed := probe.probedAddresses()
	if len(probed) != 4 {
		t.Fatalf("probed %d addresses, want 4", len(probed))
	}
	wantProbed := map[string]bool{"login-addr": true, "char-addr": true, "map-addr": true, "web-addr": true}
	for _, address := range probed {
		if !wantProbed[address] {
			t.Errorf("unexpected probed address %q", address)
		}
	}
}

func TestStatusPoller_RefreshOnce_LogsFailedProbesOnly(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	probe := &fakePortProbe{results: map[string]probeOutcome{
		"login-addr": {up: true},
		"char-addr":  {err: errors.New("connection refused")},
		"map-addr":   {up: true},
		"web-addr":   {up: true},
	}}
	logger, buffer := captureLogger()
	p := NewStatusPoller(StatusPollerConfig{
		Probe:        probe,
		Logger:       logger,
		Now:          fixedNow(now),
		LoginAddress: "login-addr",
		CharAddress:  "char-addr",
		MapAddress:   "map-addr",
		WebAddress:   "web-addr",
	})

	p.RefreshOnce(context.Background())

	out := buffer.String()
	if !strings.Contains(out, "metric: server probe failed") {
		t.Errorf("missing probe-failure log, got: %q", out)
	}
	if !strings.Contains(out, "char-addr") {
		t.Errorf("log missing failed address, got: %q", out)
	}
	if strings.Contains(out, "login-addr") {
		t.Errorf("logged a successful probe address: %q", out)
	}
}

func TestStatusPoller_RefreshOnce_SkipsLoggingWhenContextCancelled(t *testing.T) {
	t.Parallel()
	probe := &fakePortProbe{results: map[string]probeOutcome{
		"a": {err: errors.New("refused")},
		"b": {err: errors.New("refused")},
		"c": {err: errors.New("refused")},
		"d": {err: errors.New("refused")},
	}}
	logger, buffer := captureLogger()
	p := NewStatusPoller(StatusPollerConfig{
		Probe:        probe,
		Logger:       logger,
		Now:          fixedNow(time.Now()),
		LoginAddress: "a",
		CharAddress:  "b",
		MapAddress:   "c",
		WebAddress:   "d",
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p.RefreshOnce(ctx)

	if strings.Contains(buffer.String(), "server probe failed") {
		t.Errorf("logged probe failures despite cancelled context: %q", buffer.String())
	}
}

func TestStatusPoller_NewStatusPoller_Defaults(t *testing.T) {
	t.Parallel()
	p := NewStatusPoller(StatusPollerConfig{})

	if p.cfg.Logger == nil {
		t.Errorf("Logger not defaulted")
	}
	if p.cfg.Now == nil {
		t.Errorf("Now not defaulted")
	}
}
