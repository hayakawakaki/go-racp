package security

import (
	"fmt"
	"net"
	"net/http"
	"slices"
	"strings"
)

func ParseTrustedProxies(cidrs []string) ([]*net.IPNet, error) {
	parsed := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("security.ParseTrustedProxies: invalid CIDR %q: %w", cidr, err)
		}
		parsed = append(parsed, network)
	}

	return parsed, nil
}

func ClientIP(r *http.Request, trustedProxies []*net.IPNet) net.IP {
	ip := remoteIP(r.RemoteAddr)
	if ip == nil || !ipInCIDRs(ip, trustedProxies) {
		return ip
	}

	forwardedFor := r.Header.Get("X-Forwarded-For")
	if forwardedFor == "" {
		return ip
	}

	hops := strings.Split(forwardedFor, ",")
	for _, hop := range slices.Backward(hops) {
		candidate := net.ParseIP(strings.TrimSpace(hop))
		if candidate == nil {
			return ip
		}
		if !ipInCIDRs(candidate, trustedProxies) {
			return candidate
		}
	}

	return ip
}

func ipInCIDRs(ip net.IP, cidrs []*net.IPNet) bool {
	if ip == nil {
		return false
	}

	for _, network := range cidrs {
		if network.Contains(ip) {
			return true
		}
	}

	return false
}

func remoteIP(remoteAddr string) net.IP {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}

	return net.ParseIP(host)
}
