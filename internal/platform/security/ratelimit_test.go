package security

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/server/config"
)

func TestNewRateLimiter_Validation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		wantErr        string
		trustedProxies []string
		rule           config.RateLimitRule
	}{
		{
			name:    "rate per minute zero rejected",
			rule:    config.RateLimitRule{RatePerMinute: 0, Burst: 5},
			wantErr: "RatePerMinute must be > 0",
		},
		{
			name:    "rate per minute negative rejected",
			rule:    config.RateLimitRule{RatePerMinute: -2, Burst: 5},
			wantErr: "RatePerMinute must be > 0",
		},
		{
			name:    "burst zero rejected",
			rule:    config.RateLimitRule{RatePerMinute: 10, Burst: 0},
			wantErr: "Burst must be > 0",
		},
		{
			name:    "burst negative rejected",
			rule:    config.RateLimitRule{RatePerMinute: 10, Burst: -1},
			wantErr: "Burst must be > 0",
		},
		{
			name:           "invalid CIDR rejected",
			rule:           config.RateLimitRule{RatePerMinute: 10, Burst: 5},
			trustedProxies: []string{"not-a-cidr"},
			wantErr:        `invalid trusted CIDR "not-a-cidr"`,
		},
		{
			name: "valid options succeed",
			rule: config.RateLimitRule{RatePerMinute: 10, Burst: 5},
		},
		{
			name:           "valid CIDR list succeeds",
			rule:           config.RateLimitRule{RatePerMinute: 10, Burst: 5},
			trustedProxies: []string{"10.0.0.0/8", "2001:db8::/32"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			limiter, err := NewRateLimiter(RateLimiterOptions{
				Name:           "testrule",
				Rule:           tt.rule,
				TrustedProxies: tt.trustedProxies,
			})

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if limiter == nil {
					t.Fatal("limiter is nil on success")
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestRateLimiter_BurstThenReject(t *testing.T) {
	t.Parallel()

	limiter := newTestLimiter(t, RateLimiterOptions{
		Rule: config.RateLimitRule{RatePerMinute: 60, Burst: 3},
	})
	handler := limiter.Middleware(okHandler())

	statuses := make([]int, 0, 4)
	for range 4 {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, postRequest("1.2.3.4:5000"))
		statuses = append(statuses, recorder.Code)
	}

	want := []int{http.StatusOK, http.StatusOK, http.StatusOK, http.StatusTooManyRequests}
	for index, status := range statuses {
		if status != want[index] {
			t.Errorf("request %d status = %d, want %d", index+1, status, want[index])
		}
	}
}

func TestRateLimiter_RejectResponse(t *testing.T) {
	t.Parallel()

	limiter := newTestLimiter(t, RateLimiterOptions{
		Rule: config.RateLimitRule{RatePerMinute: 60, Burst: 1},
	})
	handler := limiter.Middleware(okHandler())

	for range 2 {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, postRequest("1.2.3.4:5000"))
		if recorder.Code == http.StatusTooManyRequests {
			assertRejectHeaders(t, recorder)
			if body := recorder.Body.String(); !strings.Contains(body, "you are being rate limited") {
				t.Errorf("body = %q, want substring %q", body, "you are being rate limited")
			}
			return
		}
	}

	t.Fatal("no request was rejected within burst budget")
}

func TestRateLimiter_CustomRejectFunc(t *testing.T) {
	t.Parallel()

	called := false
	limiter := newTestLimiter(t, RateLimiterOptions{
		Rule: config.RateLimitRule{RatePerMinute: 60, Burst: 1},
		Reject: func(w http.ResponseWriter, _ *http.Request) {
			called = true
			if got := w.Header().Get("Retry-After"); got == "" {
				t.Errorf("Retry-After not set before RejectFunc ran")
			}
			if got := w.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control = %q, want %q", got, "no-store")
			}
			w.WriteHeader(http.StatusTeapot)
		},
	})
	handler := limiter.Middleware(okHandler())

	var lastCode int
	for range 2 {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, postRequest("1.2.3.4:5000"))
		lastCode = recorder.Code
	}

	if !called {
		t.Errorf("RejectFunc was not called")
	}
	if lastCode != http.StatusTeapot {
		t.Errorf("status = %d, want %d (custom RejectFunc)", lastCode, http.StatusTeapot)
	}
}

func TestRateLimiter_LogsOnReject(t *testing.T) {
	t.Parallel()

	buffer := &bytes.Buffer{}
	limiter := newTestLimiter(t, RateLimiterOptions{
		Name:   "Account.Login",
		Rule:   config.RateLimitRule{RatePerMinute: 60, Burst: 1},
		Logger: slog.New(slog.NewJSONHandler(buffer, &slog.HandlerOptions{Level: slog.LevelWarn})),
	})
	handler := limiter.Middleware(okHandler())

	for range 2 {
		handler.ServeHTTP(httptest.NewRecorder(), postRequest("9.9.9.9:5000"))
	}

	record := decodeLastLog(t, buffer)
	if got := record["msg"]; got != "rate limit exceeded" {
		t.Errorf("msg = %v, want %q", got, "rate limit exceeded")
	}
	if got := record["rule"]; got != "Account.Login" {
		t.Errorf("rule = %v, want %q", got, "Account.Login")
	}
	if got := record["key"]; got != "ip:9.9.9.9" {
		t.Errorf("key = %v, want %q", got, "ip:9.9.9.9")
	}
	if got := record["method"]; got != http.MethodPost {
		t.Errorf("method = %v, want %q", got, http.MethodPost)
	}
	if got := record["path"]; got != "/login" {
		t.Errorf("path = %v, want %q", got, "/login")
	}
	if _, ok := record["retry_after_s"]; !ok {
		t.Errorf("retry_after_s missing from log: %v", record)
	}
}

func TestRateLimiter_ResolveKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		keyFn   KeyFunc
		remote  string
		wantKey string
	}{
		{
			name:    "no KeyFunc keys by IP",
			remote:  "1.2.3.4:5000",
			wantKey: "ip:1.2.3.4",
		},
		{
			name:    "KeyFunc returning user id keys by user",
			keyFn:   func(*http.Request) string { return "42" },
			remote:  "1.2.3.4:5000",
			wantKey: "u:42",
		},
		{
			name:    "empty KeyFunc result falls back to IP",
			keyFn:   func(*http.Request) string { return "" },
			remote:  "1.2.3.4:5000",
			wantKey: "ip:1.2.3.4",
		},
		{
			name:    "IPv6 falls back to /64 mask",
			remote:  "[2001:db8:1234:abcd::1]:5000",
			wantKey: "ip:2001:db8:1234:abcd::/64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			limiter := newTestLimiter(t, RateLimiterOptions{
				Rule:    config.RateLimitRule{RatePerMinute: 60, Burst: 1},
				KeyFunc: tt.keyFn,
			})

			got := limiter.resolveKey(postRequest(tt.remote))
			if got != tt.wantKey {
				t.Errorf("resolveKey = %q, want %q", got, tt.wantKey)
			}
		})
	}
}

func TestRateLimiter_RealClientIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		remote         string
		forwardedFor   string
		want           string
		trustedProxies []string
	}{
		{
			name:           "remote untrusted ignores any XFF",
			remote:         "5.6.7.8:1234",
			forwardedFor:   "9.9.9.9",
			trustedProxies: []string{"10.0.0.0/8"},
			want:           "5.6.7.8",
		},
		{
			name:           "remote trusted with no XFF returns remote",
			remote:         "10.0.0.5:1234",
			trustedProxies: []string{"10.0.0.0/8"},
			want:           "10.0.0.5",
		},
		{
			name:           "single untrusted hop is honored",
			remote:         "10.0.0.5:1234",
			forwardedFor:   "9.9.9.9",
			trustedProxies: []string{"10.0.0.0/8"},
			want:           "9.9.9.9",
		},
		{
			name:           "walk past trusted hop",
			remote:         "10.0.0.5:1234",
			forwardedFor:   "8.8.8.8, 10.0.0.6",
			trustedProxies: []string{"10.0.0.0/8"},
			want:           "8.8.8.8",
		},
		{
			name:           "malformed hop returns remote",
			remote:         "10.0.0.5:1234",
			forwardedFor:   "garbage",
			trustedProxies: []string{"10.0.0.0/8"},
			want:           "10.0.0.5",
		},
		{
			name:           "all hops trusted returns remote",
			remote:         "10.0.0.5:1234",
			forwardedFor:   "10.0.0.6, 10.0.0.7",
			trustedProxies: []string{"10.0.0.0/8"},
			want:           "10.0.0.5",
		},
		{
			name:           "IPv6 hop honored when remote trusted",
			remote:         "10.0.0.5:1234",
			forwardedFor:   "2001:db8::1",
			trustedProxies: []string{"10.0.0.0/8"},
			want:           "2001:db8::1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			limiter := newTestLimiter(t, RateLimiterOptions{
				Rule:           config.RateLimitRule{RatePerMinute: 60, Burst: 1},
				TrustedProxies: tt.trustedProxies,
			})

			req := postRequest(tt.remote)
			if tt.forwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.forwardedFor)
			}

			got := limiter.realClientIP(req)
			if got == nil {
				t.Fatalf("realClientIP returned nil for %s", tt.remote)
			}
			if got.String() != tt.want {
				t.Errorf("realClientIP = %q, want %q", got.String(), tt.want)
			}
		})
	}
}

func TestNormalizeIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
		ip   net.IP
	}{
		{name: "nil returns unknown", ip: nil, want: "unknown"},
		{name: "IPv4 returns dotted-quad", ip: net.ParseIP("1.2.3.4"), want: "1.2.3.4"},
		{name: "IPv4-mapped IPv6 collapses to v4", ip: net.ParseIP("::ffff:1.2.3.4"), want: "1.2.3.4"},
		{name: "IPv6 masked to /64", ip: net.ParseIP("2001:db8:1234:abcd::1"), want: "2001:db8:1234:abcd::/64"},
		{name: "IPv6 same /64 prefix yields same key", ip: net.ParseIP("2001:db8:1234:abcd:ffff::"), want: "2001:db8:1234:abcd::/64"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := normalizeIP(tt.ip); got != tt.want {
				t.Errorf("normalizeIP(%v) = %q, want %q", tt.ip, got, tt.want)
			}
		})
	}
}

func TestRateLimiter_LRU_MoveToFrontOnAccess(t *testing.T) {
	t.Parallel()

	limiter := newTestLimiter(t, RateLimiterOptions{
		Rule: config.RateLimitRule{RatePerMinute: 60, Burst: 1},
	})

	limiter.bucket("a")
	limiter.bucket("b")
	limiter.bucket("c")

	if got := listKeysFromHead(limiter); !equalKeys(got, []string{"c", "b", "a"}) {
		t.Fatalf("after inserts, order = %v, want [c b a]", got)
	}

	limiter.bucket("a")

	if got := listKeysFromHead(limiter); !equalKeys(got, []string{"a", "c", "b"}) {
		t.Errorf("after re-access of a, order = %v, want [a c b]", got)
	}
	if limiter.head.key != "a" {
		t.Errorf("head = %q, want %q", limiter.head.key, "a")
	}
	if limiter.tail.key != "b" {
		t.Errorf("tail = %q, want %q", limiter.tail.key, "b")
	}
}

func TestRateLimiter_EvictOldestLocked(t *testing.T) {
	t.Parallel()

	limiter := newTestLimiter(t, RateLimiterOptions{
		Rule: config.RateLimitRule{RatePerMinute: 60, Burst: 1},
	})

	limiter.bucket("a")
	limiter.bucket("b")
	limiter.bucket("c")

	limiter.mu.Lock()
	limiter.evictOldestLocked()
	limiter.mu.Unlock()

	if _, ok := limiter.buckets["a"]; ok {
		t.Errorf("oldest key %q still in map after eviction", "a")
	}
	if limiter.tail.key != "b" {
		t.Errorf("tail = %q, want %q", limiter.tail.key, "b")
	}
	if got := listKeysFromHead(limiter); !equalKeys(got, []string{"c", "b"}) {
		t.Errorf("order after eviction = %v, want [c b]", got)
	}
}

func TestRateLimiter_GCOnce(t *testing.T) {
	t.Parallel()

	limiter := newTestLimiter(t, RateLimiterOptions{
		Rule: config.RateLimitRule{RatePerMinute: 60, Burst: 1},
	})

	stale := limiter.bucket("stale-a")
	staleTwo := limiter.bucket("stale-b")
	fresh := limiter.bucket("fresh")

	cutoff := time.Now().Add(-5 * time.Minute).UnixNano()
	stale.lastSeen.Store(cutoff - 1)
	staleTwo.lastSeen.Store(cutoff - 1)
	fresh.lastSeen.Store(cutoff + int64(time.Minute))

	limiter.gcOnce(cutoff)

	if _, ok := limiter.buckets["stale-a"]; ok {
		t.Errorf("stale-a not pruned")
	}
	if _, ok := limiter.buckets["stale-b"]; ok {
		t.Errorf("stale-b not pruned")
	}
	if _, ok := limiter.buckets["fresh"]; !ok {
		t.Errorf("fresh entry was pruned")
	}
	if limiter.head != fresh || limiter.tail != fresh {
		t.Errorf("after pruning, head/tail should both point to fresh entry")
	}
}

func TestRateLimiter_GCOnceStopsAtFirstFresh(t *testing.T) {
	t.Parallel()

	limiter := newTestLimiter(t, RateLimiterOptions{
		Rule: config.RateLimitRule{RatePerMinute: 60, Burst: 1},
	})

	tailStale := limiter.bucket("tail-stale")
	middleFresh := limiter.bucket("middle-fresh")
	headStale := limiter.bucket("head-stale")

	cutoff := time.Now().UnixNano()
	tailStale.lastSeen.Store(cutoff - 1)
	middleFresh.lastSeen.Store(cutoff + int64(time.Minute))
	headStale.lastSeen.Store(cutoff - 1)

	limiter.gcOnce(cutoff)

	if _, ok := limiter.buckets["tail-stale"]; ok {
		t.Errorf("tail-stale should be pruned")
	}
	if _, ok := limiter.buckets["middle-fresh"]; !ok {
		t.Errorf("middle-fresh should remain")
	}
	if _, ok := limiter.buckets["head-stale"]; !ok {
		t.Errorf("head-stale should remain because gc stops at middle-fresh")
	}
}

func TestRateLimiter_RunExitsOnContextCancel(t *testing.T) {
	t.Parallel()

	limiter := newTestLimiter(t, RateLimiterOptions{
		Rule: config.RateLimitRule{RatePerMinute: 60, Burst: 1},
	})

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		limiter.Run(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not exit within 1s of ctx cancel")
	}
}

func TestWrap(t *testing.T) {
	t.Parallel()

	t.Run("no limiter for tag passes handler through unchanged", func(t *testing.T) {
		t.Parallel()

		handler := Wrap(map[string]*RateLimiter{}, "Missing.Tag", okHandler())

		for range 10 {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, postRequest("1.2.3.4:5000"))
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d on unwrapped handler", recorder.Code)
			}
		}
	})

	t.Run("matching tag wraps with limiter", func(t *testing.T) {
		t.Parallel()

		limiter := newTestLimiter(t, RateLimiterOptions{
			Rule: config.RateLimitRule{RatePerMinute: 60, Burst: 2},
		})
		handler := Wrap(map[string]*RateLimiter{"Account.Login": limiter}, "Account.Login", okHandler())

		var lastCode int
		for range 3 {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, postRequest("1.2.3.4:5000"))
			lastCode = recorder.Code
		}

		if lastCode != http.StatusTooManyRequests {
			t.Errorf("third request status = %d, want %d (wrap applied limiter)", lastCode, http.StatusTooManyRequests)
		}
	})
}

func newTestLimiter(t *testing.T, opts RateLimiterOptions) *RateLimiter {
	t.Helper()
	if opts.Name == "" {
		opts.Name = "testrule"
	}
	if opts.Rule.RatePerMinute == 0 {
		opts.Rule.RatePerMinute = 60
	}
	if opts.Rule.Burst == 0 {
		opts.Rule.Burst = 3
	}

	limiter, err := NewRateLimiter(opts)
	if err != nil {
		t.Fatalf("NewRateLimiter: %v", err)
	}

	return limiter
}

func postRequest(remote string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/login", http.NoBody)
	req.RemoteAddr = remote

	return req
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func assertRejectHeaders(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	if got := recorder.Code; got != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", got, http.StatusTooManyRequests)
	}
	if got := recorder.Header().Get("Retry-After"); got == "" {
		t.Errorf("Retry-After header missing")
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want %q", got, "no-store")
	}
}

func listKeysFromHead(limiter *RateLimiter) []string {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	keys := make([]string, 0, len(limiter.buckets))
	for entry := limiter.head; entry != nil; entry = entry.next {
		keys = append(keys, entry.key)
	}

	return keys
}

func equalKeys(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}

	return true
}

func decodeLastLog(t *testing.T, buffer *bytes.Buffer) map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(buffer.String()), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatalf("no log records emitted")
	}

	last := lines[len(lines)-1]
	var record map[string]any
	if err := json.Unmarshal([]byte(last), &record); err != nil {
		t.Fatalf("unmarshal log line %q: %v", last, err)
	}

	return record
}
