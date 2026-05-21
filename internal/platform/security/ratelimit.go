package security

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"

	"github.com/hayakawakaki/go-racp/server/config"
)

const (
	defaultMaxEntries = 2000
	defaultIdle       = 10 * time.Minute
	defaultGCInterval = 1 * time.Minute
)

type KeyFunc func(*http.Request) string

type RejectFunc func(w http.ResponseWriter, r *http.Request)

type RateLimiterOptions struct {
	Logger         *slog.Logger
	KeyFunc        KeyFunc
	Reject         RejectFunc
	Name           string
	TrustedProxies []string
	Rule           config.RateLimitRule
}

type RateLimiter struct {
	logger   *slog.Logger
	keyFn    KeyFunc
	rejectFn RejectFunc
	buckets  map[string]*bucketEntry
	head     *bucketEntry
	tail     *bucketEntry
	name     string
	trusted  []*net.IPNet
	limit    rate.Limit
	burst    int
	mu       sync.Mutex
}

type bucketEntry struct {
	limiter  *rate.Limiter
	prev     *bucketEntry
	next     *bucketEntry
	key      string
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
		name:     opts.Name,
		limit:    rate.Limit(float64(opts.Rule.RatePerMinute) / 60.0),
		burst:    opts.Rule.Burst,
		logger:   opts.Logger,
		keyFn:    opts.KeyFunc,
		rejectFn: opts.Reject,
		buckets:  make(map[string]*bucketEntry),
	}

	trusted, err := ParseTrustedProxies(opts.TrustedProxies)
	if err != nil {
		return nil, fmt.Errorf("security.NewRateLimiter: rate limiter %q: %w", opts.Name, err)
	}
	limiter.trusted = trusted

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
	w.Header().Set("Cache-Control", "no-store")

	if rl.rejectFn != nil {
		rl.rejectFn(w, r)
		return
	}

	http.Error(w, "you are being rate limited", http.StatusTooManyRequests)
}

func (rl *RateLimiter) bucket(key string) *bucketEntry {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now().UnixNano()
	if existing, ok := rl.buckets[key]; ok {
		existing.lastSeen.Store(now)
		rl.moveToFrontLocked(existing)
		return existing
	}

	if len(rl.buckets) >= defaultMaxEntries {
		rl.evictOldestLocked()
	}

	entry := &bucketEntry{limiter: rate.NewLimiter(rl.limit, rl.burst), key: key}
	entry.lastSeen.Store(now)
	rl.buckets[key] = entry
	rl.pushFrontLocked(entry)

	return entry
}

func (rl *RateLimiter) pushFrontLocked(entry *bucketEntry) {
	entry.prev = nil
	entry.next = rl.head
	if rl.head != nil {
		rl.head.prev = entry
	}
	rl.head = entry
	if rl.tail == nil {
		rl.tail = entry
	}
}

func (rl *RateLimiter) unlinkLocked(entry *bucketEntry) {
	if entry.prev != nil {
		entry.prev.next = entry.next
	} else {
		rl.head = entry.next
	}
	if entry.next != nil {
		entry.next.prev = entry.prev
	} else {
		rl.tail = entry.prev
	}
	entry.prev = nil
	entry.next = nil
}

func (rl *RateLimiter) moveToFrontLocked(entry *bucketEntry) {
	if rl.head == entry {
		return
	}
	rl.unlinkLocked(entry)
	rl.pushFrontLocked(entry)
}

func (rl *RateLimiter) evictOldestLocked() {
	if rl.tail == nil {
		return
	}
	oldest := rl.tail
	rl.unlinkLocked(oldest)
	delete(rl.buckets, oldest.key)
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

	for rl.tail != nil && rl.tail.lastSeen.Load() < cutoff {
		stale := rl.tail
		rl.unlinkLocked(stale)
		delete(rl.buckets, stale.key)
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
	return ClientIP(r, rl.trusted)
}

func normalizeIP(ip net.IP) string {
	if ip == nil {
		return "unknown"
	}
	if v4 := ip.To4(); v4 != nil {
		return v4.String()
	}

	masked := ip.Mask(net.CIDRMask(64, 128))

	return masked.String() + "/64"
}
