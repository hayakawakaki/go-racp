package infra

import (
	"context"
	"fmt"
	"net"
	"time"
)

type TCPProbe struct {
	dialer net.Dialer
}

func NewTCPProbe(timeout time.Duration) *TCPProbe {
	return &TCPProbe{dialer: net.Dialer{Timeout: timeout}}
}

func (p *TCPProbe) Probe(ctx context.Context, address string) (bool, error) {
	conn, err := p.dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return false, fmt.Errorf("infra.TCPProbe.Probe: %w", err)
	}
	_ = conn.Close()
	return true, nil
}
