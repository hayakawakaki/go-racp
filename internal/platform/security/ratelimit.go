package security

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"

	"github.com/hayakawakaki/go-racp/server/config"
)

const (
	defaultMaxEntries = 50_000
	defaultIdle       = 10 * time.Minute
	defaultGCInterval = 1 * time.Minute
)

type KeyFunc func(*http.Request) string

type RateLimiterOptions struct {
	Logger         *slog.Logger
	KeyFunc        KeyFunc
	Name           string
	TrustedProxies []string
	Rule           config.RateLimitRule
}

type RateLimiter struct {
	logger  *slog.Logger
	keyFn   KeyFunc
	buckets map[string]*bucketEntry
	name    string
	trusted []*net.IPNet
	limit   rate.Limit
	burst   int
	mu      sync.Mutex
}

type bucketEntry struct {
	limiter  *rate.Limiter
	lastSeen atomic.Int64
}

func NewRateLimiter(opts RateLimiterOptions) (*RateLimiter, error) {
	if opts.Rule.RatePerMinute <= 0 {
		return nil, fmt.Errorf("security.NewRateLimiter: rate limiter %q: RatePerMinute must be > 0", opts.Name)
	}

	if opts.Rule.Burst <= 0 {
		return nil, fmt.Errorf("security.NewRateLimiter: rate limiter %q: Burst must be > 0", opts.Name)
	}

	limiter := &RateLimiter{
		name:    opts.Name,
		limit:   rate.Limit(float64(opts.Rule.RatePerMinute) / 60.0),
		burst:   opts.Rule.Burst,
		logger:  opts.Logger,
		keyFn:   opts.KeyFunc,
		buckets: make(map[string]*bucketEntry),
	}

	for _, cidr := range opts.TrustedProxies {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("security.NewRateLimiter: rate limiter %q: invalid trusted CIDR %q: %w", opts.Name, cidr, err)
		}
		limiter.trusted = append(limiter.trusted, network)
	}

	return limiter, nil
}

func Wrap(limiters map[string]*RateLimiter, name string, next http.Handler) http.Handler {
	if limiter, ok := limiters[name]; ok {
		return limiter.Middleware(next)
	}

	return next
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := rl.resolveKey(r)
		entry := rl.bucket(key)

		reservation := entry.limiter.Reserve()
		if delay := reservation.Delay(); delay > 0 {
			reservation.Cancel()
			rl.reject(w, r, key, delay)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) reject(w http.ResponseWriter, r *http.Request, key string, retry time.Duration) {
	seconds := max(int(retry.Round(time.Second).Seconds()), 1)

	if rl.logger != nil {
		rl.logger.Warn("rate limit exceeded",
			"rule", rl.name,
			"key", key,
			"method", r.Method,
			"path", r.URL.Path,
			"retry_after_s", seconds,
		)
	}

	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	http.Error(w, "too many requests", http.StatusTooManyRequests)
}

func (rl *RateLimiter) bucket(key string) *bucketEntry {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now().UnixNano()
	if existing, ok := rl.buckets[key]; ok {
		existing.lastSeen.Store(now)
		return existing
	}

	if len(rl.buckets) >= defaultMaxEntries {
		rl.evictOldestLocked()
	}

	entry := &bucketEntry{limiter: rate.NewLimiter(rl.limit, rl.burst)}
	entry.lastSeen.Store(now)
	rl.buckets[key] = entry

	return entry
}

func (rl *RateLimiter) evictOldestLocked() {
	var (
		oldestKey string
		oldestAt  int64
	)
	for key, entry := range rl.buckets {
		lastSeen := entry.lastSeen.Load()
		if oldestKey == "" || lastSeen < oldestAt {
			oldestAt = lastSeen
			oldestKey = key
		}
	}

	if oldestKey != "" {
		delete(rl.buckets, oldestKey)
	}
}

func (rl *RateLimiter) Run(ctx context.Context) {
	ticker := time.NewTicker(defaultGCInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rl.gcOnce(time.Now().Add(-defaultIdle).UnixNano())
		}
	}
}

func (rl *RateLimiter) gcOnce(cutoff int64) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	for key, entry := range rl.buckets {
		if entry.lastSeen.Load() < cutoff {
			delete(rl.buckets, key)
		}
	}
}

func (rl *RateLimiter) resolveKey(r *http.Request) string {
	if rl.keyFn != nil {
		if key := rl.keyFn(r); key != "" {
			return "u:" + key
		}
	}

	return "ip:" + rl.clientIPKey(r)
}

func (rl *RateLimiter) clientIPKey(r *http.Request) string {
	return normalizeIP(rl.realClientIP(r))
}

func (rl *RateLimiter) realClientIP(r *http.Request) net.IP {
	remote := remoteIP(r.RemoteAddr)
	if remote == nil || !rl.isTrusted(remote) {
		return remote
	}

	forwardedFor := r.Header.Get("X-Forwarded-For")
	if forwardedFor == "" {
		return remote
	}

	hops := strings.Split(forwardedFor, ",")
	for _, hop := range slices.Backward(hops) {
		candidate := net.ParseIP(strings.TrimSpace(hop))
		if candidate == nil {
			return remote
		}
		if !rl.isTrusted(candidate) {
			return candidate
		}
	}

	return remote
}

func (rl *RateLimiter) isTrusted(address net.IP) bool {
	if address == nil {
		return false
	}

	for _, network := range rl.trusted {
		if network.Contains(address) {
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

func normalizeIP(address net.IP) string {
	if address == nil {
		return "unknown"
	}
	if v4 := address.To4(); v4 != nil {
		return v4.String()
	}

	masked := address.Mask(net.CIDRMask(64, 128))

	return masked.String() + "/64"
}
