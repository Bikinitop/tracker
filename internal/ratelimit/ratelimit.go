// Package ratelimit provides an in-memory, per-key token-bucket rate limiter.
//
// The Limiter interface is the seam that lets a distributed backend (e.g.
// Redis) replace the in-memory implementation later without touching handlers.
package ratelimit

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Limiter decides whether a request keyed by a string (e.g. client IP) is
// allowed under the configured rate.
type Limiter interface {
	Allow(key string) bool
}

type entry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// IPRateLimiter applies a per-key token bucket, held in process memory. A
// background goroutine evicts buckets idle longer than ttl to bound memory.
type IPRateLimiter struct {
	mu       sync.Mutex
	entries  map[string]*entry
	rate     rate.Limit
	burst    int
	ttl      time.Duration
	now      func() time.Time
	stop     chan struct{}
	stopOnce sync.Once
}

// Option configures an IPRateLimiter.
type Option func(*IPRateLimiter)

// WithClock overrides the time source (used by tests for deterministic eviction).
func WithClock(now func() time.Time) Option {
	return func(l *IPRateLimiter) { l.now = now }
}

// NewIPRateLimiter creates a limiter granting r tokens/sec with the given burst
// per key, evicting buckets idle longer than ttl. Call Stop to release the
// cleanup goroutine.
func NewIPRateLimiter(r rate.Limit, burst int, ttl time.Duration, opts ...Option) *IPRateLimiter {
	l := &IPRateLimiter{
		entries: make(map[string]*entry),
		rate:    r,
		burst:   burst,
		ttl:     ttl,
		now:     time.Now,
		stop:    make(chan struct{}),
	}
	for _, opt := range opts {
		opt(l)
	}
	// Run cleanup more often than the TTL so idle entries are evicted promptly
	// (rather than surviving up to ~2x TTL).
	go l.cleanupLoop(ttl / 2)
	return l
}

// Allow reports whether a request for key may proceed, consuming a token.
func (l *IPRateLimiter) Allow(key string) bool {
	l.mu.Lock()
	e, ok := l.entries[key]
	if !ok {
		e = &entry{limiter: rate.NewLimiter(l.rate, l.burst)}
		l.entries[key] = e
	}
	e.lastSeen = l.now()
	lim := e.limiter
	l.mu.Unlock()
	return lim.Allow()
}

// Stop terminates the background cleanup goroutine. Safe to call multiple times.
func (l *IPRateLimiter) Stop() {
	l.stopOnce.Do(func() { close(l.stop) })
}

func (l *IPRateLimiter) cleanupLoop(interval time.Duration) {
	if interval <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			l.cleanup()
		case <-l.stop:
			return
		}
	}
}

func (l *IPRateLimiter) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := l.now().Add(-l.ttl)
	for k, e := range l.entries {
		if e.lastSeen.Before(cutoff) {
			delete(l.entries, k)
		}
	}
}

func (l *IPRateLimiter) len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.entries)
}
