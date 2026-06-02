package ratelimit

import (
	"sync"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

// len reports the number of tracked entries (test-only helper).
func (l *IPRateLimiter) len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.entries)
}

func TestNewIPRateLimiter_ClampsInvalidValues(t *testing.T) {
	// burst 0 would otherwise reject the very first request; it is clamped to 1.
	// A non-positive ttl is clamped so eviction stays enabled. Must not panic.
	l := NewIPRateLimiter(rate.Limit(1), 0, -5*time.Second)
	defer l.Stop()

	if !l.Allow("1.1.1.1") {
		t.Errorf("expected first request allowed after burst clamped to >= 1")
	}
}

func TestIPRateLimiter_AllowsUpToBurstThenBlocks(t *testing.T) {
	// rate of 1/sec but burst of 3: first 3 calls succeed immediately,
	// the 4th has no token yet (no meaningful time has elapsed).
	l := NewIPRateLimiter(rate.Limit(1), 3, time.Minute)
	defer l.Stop()

	for i := 0; i < 3; i++ {
		if !l.Allow("1.1.1.1") {
			t.Fatalf("call %d: expected allow within burst", i)
		}
	}
	if l.Allow("1.1.1.1") {
		t.Errorf("expected block after burst exhausted")
	}
}

func TestIPRateLimiter_KeysAreIsolated(t *testing.T) {
	l := NewIPRateLimiter(rate.Limit(1), 1, time.Minute)
	defer l.Stop()

	if !l.Allow("1.1.1.1") {
		t.Fatalf("first key should be allowed")
	}
	if !l.Allow("2.2.2.2") {
		t.Errorf("second key should have its own bucket")
	}
}

func TestIPRateLimiter_EvictsIdleEntries(t *testing.T) {
	current := time.Unix(1000, 0)
	l := NewIPRateLimiter(rate.Limit(10), 10, time.Minute, WithClock(func() time.Time { return current }))
	defer l.Stop()

	l.Allow("1.1.1.1")
	if got := l.len(); got != 1 {
		t.Fatalf("expected 1 entry, got %d", got)
	}

	current = current.Add(2 * time.Minute) // exceed TTL
	l.cleanup()

	if got := l.len(); got != 0 {
		t.Errorf("expected idle entry evicted, got %d entries", got)
	}
}

func TestIPRateLimiter_StopIsIdempotent(t *testing.T) {
	l := NewIPRateLimiter(rate.Limit(1), 1, time.Minute)
	l.Stop()
	l.Stop() // must not panic
}

func TestIPRateLimiter_ConcurrentAllowIsRaceFree(t *testing.T) {
	l := NewIPRateLimiter(rate.Limit(1000), 1000, time.Minute)
	defer l.Stop()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l.Allow("1.1.1.1")
		}()
	}
	wg.Wait()
}
