package infra

import (
	"context"
	"net"
	"time"
)

type TCPProbe struct {
	dialer net.Dialer
}

func NewTCPProbe(timeout time.Duration) *TCPProbe {
	return &TCPProbe{dialer: net.Dialer{Timeout: timeout}}
}

func (p *TCPProbe) Probe(ctx context.Context, address string) bool {
	conn, err := p.dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
