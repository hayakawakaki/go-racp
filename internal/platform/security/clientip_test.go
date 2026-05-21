package security

import (
	"net"
	"net/http"
	"strings"
	"testing"
)

func mustCIDR(t *testing.T, cidr string) *net.IPNet {
	t.Helper()
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatalf("ParseCIDR(%q): %v", cidr, err)
	}
	return network
}

func TestParseTrustedProxies(t *testing.T) {
	t.Parallel()

	t.Run("empty input returns empty slice", func(t *testing.T) {
		t.Parallel()
		got, err := ParseTrustedProxies(nil)
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if len(got) != 0 {
			t.Errorf("len = %d, want 0", len(got))
		}
	})

	t.Run("valid CIDR list parses", func(t *testing.T) {
		t.Parallel()
		got, err := ParseTrustedProxies([]string{"127.0.0.1/32", "10.0.0.0/8", "::1/128"})
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if len(got) != 3 {
			t.Errorf("len = %d, want 3", len(got))
		}
	})

	t.Run("invalid CIDR returns error", func(t *testing.T) {
		t.Parallel()
		_, err := ParseTrustedProxies([]string{"not-a-cidr"})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("error names the bad entry", func(t *testing.T) {
		t.Parallel()
		_, err := ParseTrustedProxies([]string{"127.0.0.1/32", "garbage"})
		if err == nil {
			t.Fatal("expected error")
		}
		if want := `"garbage"`; !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err.Error(), want)
		}
	})
}

func TestClientIP(t *testing.T) {
	t.Parallel()

	loopbackProxy := []*net.IPNet{mustCIDR(t, "127.0.0.1/32")}
	wideRangeProxy := []*net.IPNet{mustCIDR(t, "10.0.0.0/8")}

	//nolint:govet // fine for test
	tests := []struct {
		trustedProxies []*net.IPNet
		name           string
		remoteAddr     string
		xff            string
		wantIP         string
	}{
		{
			name:       "no proxies trusts only remote",
			remoteAddr: "203.0.113.5:12345",
			xff:        "198.51.100.7",
			wantIP:     "203.0.113.5",
		},
		{
			name:           "untrusted remote ignores XFF",
			remoteAddr:     "203.0.113.5:12345",
			xff:            "198.51.100.7",
			trustedProxies: loopbackProxy,
			wantIP:         "203.0.113.5",
		},
		{
			name:           "trusted remote with empty XFF returns remote",
			remoteAddr:     "127.0.0.1:5555",
			trustedProxies: loopbackProxy,
			wantIP:         "127.0.0.1",
		},
		{
			name:           "trusted remote returns last XFF hop",
			remoteAddr:     "127.0.0.1:5555",
			xff:            "198.51.100.7",
			trustedProxies: loopbackProxy,
			wantIP:         "198.51.100.7",
		},
		{
			name:           "walks back through chain of trusted hops",
			remoteAddr:     "10.0.0.1:80",
			xff:            "198.51.100.7, 10.0.0.99",
			trustedProxies: wideRangeProxy,
			wantIP:         "198.51.100.7",
		},
		{
			name:           "stops at first untrusted hop walking backward",
			remoteAddr:     "10.0.0.1:80",
			xff:            "198.51.100.7, 203.0.113.99, 10.0.0.99",
			trustedProxies: wideRangeProxy,
			wantIP:         "203.0.113.99",
		},
		{
			name:           "malformed XFF hop falls back to remote",
			remoteAddr:     "127.0.0.1:80",
			xff:            "not-an-ip",
			trustedProxies: loopbackProxy,
			wantIP:         "127.0.0.1",
		},
		{
			name:           "ipv6 trusted remote returns XFF",
			remoteAddr:     "[::1]:80",
			xff:            "2001:db8::1",
			trustedProxies: []*net.IPNet{mustCIDR(t, "::1/128")},
			wantIP:         "2001:db8::1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httpRequestWith(tt.remoteAddr, tt.xff)
			got := ClientIP(req, tt.trustedProxies)
			if got == nil {
				t.Fatalf("ClientIP returned nil")
			}
			if got.String() != tt.wantIP {
				t.Errorf("ClientIP = %q, want %q", got.String(), tt.wantIP)
			}
		})
	}
}

func TestClientIP_RemoteAddrWithoutPortStillResolves(t *testing.T) {
	t.Parallel()
	req := &http.Request{RemoteAddr: "203.0.113.5", Header: make(http.Header)}
	got := ClientIP(req, nil)
	if got == nil || got.String() != "203.0.113.5" {
		t.Errorf("ClientIP = %v, want 203.0.113.5", got)
	}
}

func TestClientIP_InvalidRemoteAddrReturnsNil(t *testing.T) {
	t.Parallel()
	req := &http.Request{RemoteAddr: "garbage", Header: make(http.Header)}
	if got := ClientIP(req, nil); got != nil {
		t.Errorf("ClientIP = %v, want nil for unparseable address", got)
	}
}

func httpRequestWith(remoteAddr, xff string) *http.Request {
	req := &http.Request{RemoteAddr: remoteAddr, Header: make(http.Header)}
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	return req
}
