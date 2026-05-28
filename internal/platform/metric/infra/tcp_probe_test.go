package infra

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

func TestTCPProbe_Probe_OpenPortReturnsTrue(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = listener.Close() }()

	probe := NewTCPProbe(time.Second)
	up, err := probe.Probe(context.Background(), listener.Addr().String())
	if err != nil {
		t.Fatalf("Probe on open port returned error: %v", err)
	}
	if !up {
		t.Errorf("Probe(open port) = false, want true")
	}
}

func TestTCPProbe_Probe_ClosedPortReturnsFalse(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	address := listener.Addr().String()
	if closeErr := listener.Close(); closeErr != nil {
		t.Fatalf("close: %v", closeErr)
	}

	probe := NewTCPProbe(200 * time.Millisecond)
	up, err := probe.Probe(context.Background(), address)
	if up {
		t.Errorf("Probe(closed port) = true, want false")
	}
	if err == nil || !strings.Contains(err.Error(), "infra.TCPProbe.Probe") {
		t.Errorf("error = %v, want wrapped with infra.TCPProbe.Probe", err)
	}
}

func TestTCPProbe_Probe_CancelledContextReturnsError(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = listener.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	probe := NewTCPProbe(time.Second)
	up, err := probe.Probe(ctx, listener.Addr().String())
	if up {
		t.Errorf("Probe(cancelled ctx) = true, want false")
	}
	if err == nil {
		t.Errorf("Probe(cancelled ctx) error = nil, want error")
	}
}
