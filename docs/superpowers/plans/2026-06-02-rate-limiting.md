# Rate Limiting & Circuit Breaker Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add per-IP token-bucket rate limiting (429) and a NATS-publish circuit breaker (503) to the tracker.

**Architecture:** Per-IP limiting is HTTP middleware wrapping only `/track`. The circuit breaker is a decorator over `EventPublisher` that observes publish outcomes and fast-fails when NATS is unhealthy. Both are in-memory, sit behind small interfaces, and are toggleable via env vars. Clocks are injectable so state-transition tests are deterministic (no `time.Sleep`).

**Tech Stack:** Go 1.25, `golang.org/x/time/rate` (already an indirect dep), stdlib `net/http`, `httptest`.

**Design doc:** `docs/superpowers/specs/2026-06-02-rate-limiting-design.md`

**Conventions to follow (observed in repo):**
- Table-driven tests, `httptest` for handlers, `t.Errorf` style assertions.
- Tests live beside code in the same package (`package api`, `package config`, etc.).
- Run everything with the race detector: `go test -race ./...`.

---

## Task 1: Promote `golang.org/x/time` to a direct dependency

**Files:**
- Modify: `go.mod`

- [ ] **Step 1: Add the import to the module graph**

`golang.org/x/time v0.15.0` is already listed as `// indirect`. Promote it by fetching the package we'll use:

Run: `go get golang.org/x/time/rate@v0.15.0`
Expected: `go.mod` updated; `golang.org/x/time` line no longer marked `// indirect` (it will be re-tidied later anyway).

- [ ] **Step 2: Verify the package resolves**

Run: `go list golang.org/x/time/rate`
Expected: prints `golang.org/x/time/rate` with no error.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "build: promote golang.org/x/time to direct dependency"
```

---

## Task 2: `internal/ratelimit` — Limiter interface + in-memory IP token-bucket

**Files:**
- Create: `internal/ratelimit/ratelimit.go`
- Test: `internal/ratelimit/ratelimit_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/ratelimit/ratelimit_test.go`:

```go
package ratelimit

import (
	"sync"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ratelimit/...`
Expected: FAIL — `undefined: NewIPRateLimiter` (package doesn't exist yet).

- [ ] **Step 3: Implement the limiter**

Create `internal/ratelimit/ratelimit.go`:

```go
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
	mu      sync.Mutex
	entries map[string]*entry
	rate    rate.Limit
	burst   int
	ttl     time.Duration
	now     func() time.Time
	stop    chan struct{}
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
	go l.cleanupLoop(ttl)
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

// Stop terminates the background cleanup goroutine.
func (l *IPRateLimiter) Stop() {
	close(l.stop)
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
```

- [ ] **Step 4: Run tests to verify they pass (with race detector)**

Run: `go test -race ./internal/ratelimit/...`
Expected: PASS (all 4 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/ratelimit/
git commit -m "feat(ratelimit): in-memory per-IP token-bucket limiter"
```

---

## Task 3: `internal/circuitbreaker` — 3-state breaker on publish failures

**Files:**
- Create: `internal/circuitbreaker/circuitbreaker.go`
- Test: `internal/circuitbreaker/circuitbreaker_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/circuitbreaker/circuitbreaker_test.go`:

```go
package circuitbreaker

import (
	"sync"
	"testing"
	"time"
)

func testConfig() Config {
	return Config{
		FailureRatio:   0.5,
		MinRequests:    4,
		Window:         10 * time.Second,
		OpenDuration:   5 * time.Second,
		HalfOpenProbes: 2,
	}
}

func TestBreaker_StartsClosedAndAllows(t *testing.T) {
	b := New(testConfig())
	if b.State() != StateClosed {
		t.Fatalf("expected initial state Closed")
	}
	if !b.Allow() {
		t.Errorf("closed breaker should allow")
	}
}

func TestBreaker_TripsOpenWhenFailureRatioExceeded(t *testing.T) {
	b := New(testConfig())
	// 4 requests, all failures: ratio 1.0 >= 0.5 and total >= MinRequests(4).
	for i := 0; i < 4; i++ {
		b.Record(false)
	}
	if b.State() != StateOpen {
		t.Fatalf("expected Open after failures, got %v", b.State())
	}
	if b.Allow() {
		t.Errorf("open breaker should fast-fail")
	}
}

func TestBreaker_StaysClosedBelowMinRequests(t *testing.T) {
	b := New(testConfig())
	// 3 failures only: below MinRequests floor, must not trip.
	for i := 0; i < 3; i++ {
		b.Record(false)
	}
	if b.State() != StateClosed {
		t.Errorf("expected Closed below MinRequests, got %v", b.State())
	}
}

func TestBreaker_MixedRatioAtThresholdTrips(t *testing.T) {
	b := New(testConfig())
	b.Record(true)
	b.Record(true)
	b.Record(false)
	b.Record(false) // 2/4 = 0.5 >= FailureRatio
	if b.State() != StateOpen {
		t.Errorf("expected Open at exactly the threshold, got %v", b.State())
	}
}

func TestBreaker_OpenTransitionsToHalfOpenAfterDuration(t *testing.T) {
	current := time.Unix(1000, 0)
	b := New(testConfig(), WithClock(func() time.Time { return current }))
	for i := 0; i < 4; i++ {
		b.Record(false)
	}
	if b.State() != StateOpen {
		t.Fatalf("precondition: expected Open")
	}

	current = current.Add(5 * time.Second) // == OpenDuration
	if !b.Allow() {
		t.Errorf("expected probe allowed after open duration")
	}
	if b.State() != StateHalfOpen {
		t.Errorf("expected HalfOpen after open duration, got %v", b.State())
	}
}

func TestBreaker_HalfOpenClosesAfterEnoughProbeSuccesses(t *testing.T) {
	current := time.Unix(1000, 0)
	b := New(testConfig(), WithClock(func() time.Time { return current }))
	for i := 0; i < 4; i++ {
		b.Record(false)
	}
	current = current.Add(5 * time.Second)

	// HalfOpenProbes = 2: two successful probes close the breaker.
	for i := 0; i < 2; i++ {
		if !b.Allow() {
			t.Fatalf("probe %d should be allowed in half-open", i)
		}
		b.Record(true)
	}
	if b.State() != StateClosed {
		t.Errorf("expected Closed after successful probes, got %v", b.State())
	}
}

func TestBreaker_HalfOpenReopensOnProbeFailure(t *testing.T) {
	current := time.Unix(1000, 0)
	b := New(testConfig(), WithClock(func() time.Time { return current }))
	for i := 0; i < 4; i++ {
		b.Record(false)
	}
	current = current.Add(5 * time.Second)

	if !b.Allow() {
		t.Fatalf("first probe should be allowed")
	}
	b.Record(false) // probe fails
	if b.State() != StateOpen {
		t.Errorf("expected reopen on probe failure, got %v", b.State())
	}
}

func TestBreaker_ConcurrentUseIsRaceFree(t *testing.T) {
	b := New(testConfig())
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			if b.Allow() {
				b.Record(n%2 == 0)
			}
		}(i)
	}
	wg.Wait()
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/circuitbreaker/...`
Expected: FAIL — `undefined: New` / `undefined: Config`.

- [ ] **Step 3: Implement the breaker**

Create `internal/circuitbreaker/circuitbreaker.go`:

```go
// Package circuitbreaker implements a 3-state circuit breaker used to fast-fail
// when a downstream dependency (NATS publishing) is unhealthy.
package circuitbreaker

import (
	"errors"
	"sync"
	"time"
)

// ErrCircuitOpen is returned by callers when the breaker is open.
var ErrCircuitOpen = errors.New("circuit breaker open")

// State is the breaker's current state.
type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// Config tunes the breaker.
type Config struct {
	FailureRatio   float64       // failures/total that trips the breaker
	MinRequests    int           // min samples in window before tripping
	Window         time.Duration // rolling window for failure accounting
	OpenDuration   time.Duration // time in Open before probing
	HalfOpenProbes int           // probe successes needed to close
}

// Breaker is a thread-safe circuit breaker.
type Breaker struct {
	mu  sync.Mutex
	cfg Config
	now func() time.Time

	state       State
	failures    int
	successes   int
	windowStart time.Time
	openedAt    time.Time

	halfOpenSuccesses int
	halfOpenInflight  int
}

// Option configures a Breaker.
type Option func(*Breaker)

// WithClock overrides the time source (used by tests).
func WithClock(now func() time.Time) Option {
	return func(b *Breaker) { b.now = now }
}

// New creates a Breaker in the Closed state.
func New(cfg Config, opts ...Option) *Breaker {
	b := &Breaker{
		cfg:   cfg,
		now:   time.Now,
		state: StateClosed,
	}
	for _, opt := range opts {
		opt(b)
	}
	b.windowStart = b.now()
	return b
}

// State returns the current state.
func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

// Allow reports whether a request may proceed. In Open it returns false until
// OpenDuration elapses, then transitions to HalfOpen and admits limited probes.
func (b *Breaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case StateOpen:
		if b.now().Sub(b.openedAt) >= b.cfg.OpenDuration {
			b.toHalfOpen()
			b.halfOpenInflight++
			return true
		}
		return false
	case StateHalfOpen:
		if b.halfOpenInflight >= b.cfg.HalfOpenProbes {
			return false
		}
		b.halfOpenInflight++
		return true
	default: // StateClosed
		return true
	}
}

// Record feeds the outcome of an allowed request back to the breaker.
func (b *Breaker) Record(success bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case StateHalfOpen:
		if b.halfOpenInflight > 0 {
			b.halfOpenInflight--
		}
		if !success {
			b.toOpen()
			return
		}
		b.halfOpenSuccesses++
		if b.halfOpenSuccesses >= b.cfg.HalfOpenProbes {
			b.toClosed()
		}
	case StateClosed:
		b.rollWindow()
		if success {
			b.successes++
		} else {
			b.failures++
		}
		total := b.successes + b.failures
		if total >= b.cfg.MinRequests {
			if float64(b.failures)/float64(total) >= b.cfg.FailureRatio {
				b.toOpen()
			}
		}
	case StateOpen:
		// Records while fully open are ignored; recovery is probe-driven.
	}
}

func (b *Breaker) rollWindow() {
	if b.now().Sub(b.windowStart) >= b.cfg.Window {
		b.windowStart = b.now()
		b.failures = 0
		b.successes = 0
	}
}

func (b *Breaker) toOpen() {
	b.state = StateOpen
	b.openedAt = b.now()
	b.halfOpenSuccesses = 0
	b.halfOpenInflight = 0
}

func (b *Breaker) toHalfOpen() {
	b.state = StateHalfOpen
	b.halfOpenSuccesses = 0
	b.halfOpenInflight = 0
}

func (b *Breaker) toClosed() {
	b.state = StateClosed
	b.failures = 0
	b.successes = 0
	b.windowStart = b.now()
	b.halfOpenSuccesses = 0
	b.halfOpenInflight = 0
}
```

- [ ] **Step 4: Run tests to verify they pass (with race detector)**

Run: `go test -race ./internal/circuitbreaker/...`
Expected: PASS (all 8 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/circuitbreaker/
git commit -m "feat(circuitbreaker): 3-state breaker for downstream health"
```

---

## Task 4: Config — env vars for both features

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/config/config_test.go`:

```go
func TestLoad_RateLimitDefaults(t *testing.T) {
	cfg := Load()
	if !cfg.RateLimitEnabled {
		t.Errorf("expected rate limiting enabled by default")
	}
	if cfg.RateLimitRPS != 50 {
		t.Errorf("expected default RPS 50, got %v", cfg.RateLimitRPS)
	}
	if cfg.RateLimitBurst != 100 {
		t.Errorf("expected default burst 100, got %d", cfg.RateLimitBurst)
	}
	if cfg.RateLimitIPTTL != 10*time.Minute {
		t.Errorf("expected default TTL 10m, got %v", cfg.RateLimitIPTTL)
	}
	if cfg.TrustProxy {
		t.Errorf("expected TrustProxy false by default")
	}
}

func TestLoad_CircuitBreakerDefaults(t *testing.T) {
	cfg := Load()
	if !cfg.CBEnabled {
		t.Errorf("expected circuit breaker enabled by default")
	}
	if cfg.CBFailureRatio != 0.5 {
		t.Errorf("expected default failure ratio 0.5, got %v", cfg.CBFailureRatio)
	}
	if cfg.CBMinRequests != 20 {
		t.Errorf("expected default min requests 20, got %d", cfg.CBMinRequests)
	}
	if cfg.CBWindow != 10*time.Second {
		t.Errorf("expected default window 10s, got %v", cfg.CBWindow)
	}
	if cfg.CBOpenDuration != 5*time.Second {
		t.Errorf("expected default open duration 5s, got %v", cfg.CBOpenDuration)
	}
	if cfg.CBHalfOpenProbes != 5 {
		t.Errorf("expected default half-open probes 5, got %d", cfg.CBHalfOpenProbes)
	}
}

func TestLoad_RateLimitFromEnv(t *testing.T) {
	os.Setenv("TRACKER_RATELIMIT_ENABLED", "false")
	os.Setenv("TRACKER_RATELIMIT_RPS", "100")
	os.Setenv("TRACKER_TRUST_PROXY", "true")
	defer os.Unsetenv("TRACKER_RATELIMIT_ENABLED")
	defer os.Unsetenv("TRACKER_RATELIMIT_RPS")
	defer os.Unsetenv("TRACKER_TRUST_PROXY")

	cfg := Load()
	if cfg.RateLimitEnabled {
		t.Errorf("expected rate limiting disabled")
	}
	if cfg.RateLimitRPS != 100 {
		t.Errorf("expected RPS 100, got %v", cfg.RateLimitRPS)
	}
	if !cfg.TrustProxy {
		t.Errorf("expected TrustProxy true")
	}
}

func TestLoad_InvalidValuesFallBackToDefaults(t *testing.T) {
	os.Setenv("TRACKER_RATELIMIT_RPS", "not-a-number")
	os.Setenv("TRACKER_CB_WINDOW", "garbage")
	defer os.Unsetenv("TRACKER_RATELIMIT_RPS")
	defer os.Unsetenv("TRACKER_CB_WINDOW")

	cfg := Load()
	if cfg.RateLimitRPS != 50 {
		t.Errorf("expected fallback RPS 50, got %v", cfg.RateLimitRPS)
	}
	if cfg.CBWindow != 10*time.Second {
		t.Errorf("expected fallback window 10s, got %v", cfg.CBWindow)
	}
}
```

Add `"time"` to the test file's imports.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/...`
Expected: FAIL — `cfg.RateLimitEnabled undefined`.

- [ ] **Step 3: Implement config fields and parsing**

Replace the contents of `internal/config/config.go` with:

```go
package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds application configuration.
type Config struct {
	Port    string
	NATSURL string

	// Per-IP rate limiting
	RateLimitEnabled bool
	RateLimitRPS     float64
	RateLimitBurst   int
	RateLimitIPTTL   time.Duration
	TrustProxy       bool

	// NATS-publish circuit breaker
	CBEnabled        bool
	CBFailureRatio   float64
	CBMinRequests    int
	CBWindow         time.Duration
	CBOpenDuration   time.Duration
	CBHalfOpenProbes int
}

// Load reads configuration from environment variables with defaults.
func Load() *Config {
	cfg := &Config{
		Port:    getEnv("TRACKER_PORT", "8080"),
		NATSURL: getEnv("NATS_URL", "nats://localhost:4222"),

		RateLimitEnabled: getEnvBool("TRACKER_RATELIMIT_ENABLED", true),
		RateLimitRPS:     getEnvFloat("TRACKER_RATELIMIT_RPS", 50),
		RateLimitBurst:   getEnvInt("TRACKER_RATELIMIT_BURST", 100),
		RateLimitIPTTL:   getEnvDuration("TRACKER_RATELIMIT_IP_TTL", 10*time.Minute),
		TrustProxy:       getEnvBool("TRACKER_TRUST_PROXY", false),

		CBEnabled:        getEnvBool("TRACKER_CB_ENABLED", true),
		CBFailureRatio:   getEnvFloat("TRACKER_CB_FAILURE_RATIO", 0.5),
		CBMinRequests:    getEnvInt("TRACKER_CB_MIN_REQUESTS", 20),
		CBWindow:         getEnvDuration("TRACKER_CB_WINDOW", 10*time.Second),
		CBOpenDuration:   getEnvDuration("TRACKER_CB_OPEN_DURATION", 5*time.Second),
		CBHalfOpenProbes: getEnvInt("TRACKER_CB_HALF_OPEN_PROBES", 5),
	}

	// Allow explicitly disabling NATS by setting NATS_URL=disabled or empty
	if cfg.NATSURL == "disabled" {
		cfg.NATSURL = ""
	}

	return cfg
}

func getEnv(key, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(value); err == nil {
			return n
		}
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value, ok := os.LookupEnv(key); ok {
		if f, err := strconv.ParseFloat(value, 64); err == nil {
			return f
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return defaultValue
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/...`
Expected: PASS (existing + new tests).

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat(config): rate limit and circuit breaker env vars"
```

---

## Task 5: API middleware — client IP extraction + rate-limit middleware

**Files:**
- Create: `internal/api/middleware.go`
- Test: `internal/api/middleware_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/api/middleware_test.go`:

```go
package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// stubLimiter returns a fixed verdict and records the last key it saw.
type stubLimiter struct {
	allow   bool
	lastKey string
}

func (s *stubLimiter) Allow(key string) bool {
	s.lastKey = key
	return s.allow
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("passed"))
	})
}

func TestRateLimitMiddleware_AllowsWhenUnderLimit(t *testing.T) {
	mw := RateLimitMiddleware(&stubLimiter{allow: true}, false, 1, okHandler())

	req := httptest.NewRequest(http.MethodGet, "/track?idsite=1&rec=1", nil)
	req.RemoteAddr = "9.9.9.9:1234"
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if rr.Body.String() != "passed" {
		t.Errorf("expected request to reach next handler")
	}
}

func TestRateLimitMiddleware_Returns429WhenOverLimit(t *testing.T) {
	mw := RateLimitMiddleware(&stubLimiter{allow: false}, false, 3, okHandler())

	req := httptest.NewRequest(http.MethodGet, "/track?idsite=1&rec=1", nil)
	req.RemoteAddr = "9.9.9.9:1234"
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rr.Code)
	}
	if got := rr.Header().Get("Retry-After"); got != "3" {
		t.Errorf("expected Retry-After 3, got %q", got)
	}
}

func TestRateLimitMiddleware_UsesRemoteAddrByDefault(t *testing.T) {
	stub := &stubLimiter{allow: true}
	mw := RateLimitMiddleware(stub, false, 1, okHandler())

	req := httptest.NewRequest(http.MethodGet, "/track?idsite=1&rec=1", nil)
	req.RemoteAddr = "5.6.7.8:4321"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	if stub.lastKey != "5.6.7.8" {
		t.Errorf("expected RemoteAddr host as key, got %q", stub.lastKey)
	}
}

func TestRateLimitMiddleware_UsesXForwardedForWhenTrusted(t *testing.T) {
	stub := &stubLimiter{allow: true}
	mw := RateLimitMiddleware(stub, true, 1, okHandler())

	req := httptest.NewRequest(http.MethodGet, "/track?idsite=1&rec=1", nil)
	req.RemoteAddr = "5.6.7.8:4321"
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	if stub.lastKey != "1.2.3.4" {
		t.Errorf("expected leftmost XFF as key, got %q", stub.lastKey)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/api/... -run RateLimitMiddleware`
Expected: FAIL — `undefined: RateLimitMiddleware`.

- [ ] **Step 3: Implement the middleware**

Create `internal/api/middleware.go`:

```go
package api

import (
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/bikinitop/tracker/internal/ratelimit"
)

// clientIP determines the client IP used as the rate-limit key. When trustProxy
// is true the leftmost X-Forwarded-For entry is used (only safe behind a
// trusted proxy that sets it); otherwise the connection's RemoteAddr host.
func clientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if ip := strings.TrimSpace(strings.Split(xff, ",")[0]); ip != "" {
				return ip
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// RateLimitMiddleware rejects requests whose client IP has exhausted its quota
// with 429 and a Retry-After header; otherwise it calls next.
func RateLimitMiddleware(limiter ratelimit.Limiter, trustProxy bool, retryAfterSeconds int, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow(clientIP(r, trustProxy)) {
			w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds))
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -race ./internal/api/... -run RateLimitMiddleware`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/api/middleware.go internal/api/middleware_test.go
git commit -m "feat(api): rate-limit middleware with proxy-aware client IP"
```

---

## Task 6: Router — optional rate limiter wrapping only `/track`

**Files:**
- Modify: `internal/api/router.go`
- Test: `internal/api/router_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/api/router_test.go`:

```go
func TestRouter_RateLimitedTrackReturns429(t *testing.T) {
	router := NewRouter(&noopPublisher{}, WithRateLimiter(&stubLimiter{allow: false}, false))

	req := httptest.NewRequest(http.MethodGet, "/track?idsite=1&rec=1", nil)
	req.RemoteAddr = "9.9.9.9:1111"
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 on /track, got %d", rr.Code)
	}
}

func TestRouter_HealthNotRateLimited(t *testing.T) {
	router := NewRouter(&noopPublisher{}, WithRateLimiter(&stubLimiter{allow: false}, false))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected /health to bypass rate limiting, got %d", rr.Code)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/api/... -run Router`
Expected: FAIL — `undefined: WithRateLimiter`.

- [ ] **Step 3: Implement router options**

Replace the contents of `internal/api/router.go` with:

```go
package api

import (
	"net/http"

	"github.com/bikinitop/tracker/internal/ratelimit"
)

type routerConfig struct {
	limiter    ratelimit.Limiter
	trustProxy bool
}

// RouterOption configures the router.
type RouterOption func(*routerConfig)

// WithRateLimiter wraps the /track endpoint in per-IP rate limiting.
func WithRateLimiter(limiter ratelimit.Limiter, trustProxy bool) RouterOption {
	return func(rc *routerConfig) {
		rc.limiter = limiter
		rc.trustProxy = trustProxy
	}
}

// NewRouter creates an HTTP router with all tracking endpoints. Options may
// enable rate limiting on the tracking endpoint.
func NewRouter(publisher EventPublisher, opts ...RouterOption) http.Handler {
	rc := &routerConfig{}
	for _, opt := range opts {
		opt(rc)
	}

	mux := http.NewServeMux()

	// Bikinitop branded tracking endpoint
	var trackHandler http.Handler = TrackHandler(publisher)
	if rc.limiter != nil {
		trackHandler = RateLimitMiddleware(rc.limiter, rc.trustProxy, 1, trackHandler)
	}
	mux.Handle("/track", trackHandler)

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	return mux
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -race ./internal/api/... -run Router`
Expected: PASS (existing 4 Router tests still compile via variadic options + 2 new).

- [ ] **Step 5: Commit**

```bash
git add internal/api/router.go internal/api/router_test.go
git commit -m "feat(api): optional rate limiter on /track via router options"
```

---

## Task 7: Breaker publisher decorator + handler 503 mapping

**Files:**
- Create: `internal/api/breaker_publisher.go`
- Test: `internal/api/breaker_publisher_test.go`
- Modify: `internal/api/handler.go` (imports + `processSingleRequest`)

- [ ] **Step 1: Write failing tests**

Create `internal/api/breaker_publisher_test.go`:

```go
package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bikinitop/tracker/internal/circuitbreaker"
	"github.com/bikinitop/tracker/internal/tracker"
)

// failingPublisher always errors, to drive the breaker open.
type failingPublisher struct{ calls int }

func (f *failingPublisher) PublishEvent(*tracker.Event) error {
	f.calls++
	return errors.New("nats down")
}

func breakerTestConfig() circuitbreaker.Config {
	return circuitbreaker.Config{
		FailureRatio:   0.5,
		MinRequests:    2,
		Window:         10 * time.Second,
		OpenDuration:   5 * time.Second,
		HalfOpenProbes: 1,
	}
}

func TestBreakerPublisher_ReturnsErrCircuitOpenWhenOpen(t *testing.T) {
	inner := &failingPublisher{}
	br := circuitbreaker.New(breakerTestConfig())
	pub := NewBreakerPublisher(inner, br)

	ev := &tracker.Event{SiteID: "1"}
	// Two failures trip the breaker (MinRequests=2, ratio 1.0).
	_ = pub.PublishEvent(ev)
	_ = pub.PublishEvent(ev)

	callsBefore := inner.calls
	err := pub.PublishEvent(ev)
	if !errors.Is(err, circuitbreaker.ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
	if inner.calls != callsBefore {
		t.Errorf("expected inner publisher NOT called while open")
	}
}

func TestBreakerPublisher_PassesThroughWhenClosed(t *testing.T) {
	inner := &MockPublisher{}
	br := circuitbreaker.New(breakerTestConfig())
	pub := NewBreakerPublisher(inner, br)

	if err := pub.PublishEvent(&tracker.Event{SiteID: "1"}); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if len(inner.Events) != 1 {
		t.Errorf("expected event forwarded to inner publisher")
	}
}

func TestHandler_CircuitOpenReturns503(t *testing.T) {
	inner := &failingPublisher{}
	br := circuitbreaker.New(breakerTestConfig())
	pub := NewBreakerPublisher(inner, br)
	handler := TrackHandler(pub)

	// Trip the breaker with two failing requests (these return 500).
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/track?idsite=1&rec=1", nil)
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}

	// Next request should fast-fail with 503.
	req := httptest.NewRequest(http.MethodGet, "/track?idsite=1&rec=1", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 when circuit open, got %d", rr.Code)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/api/... -run 'BreakerPublisher|CircuitOpen'`
Expected: FAIL — `undefined: NewBreakerPublisher`.

- [ ] **Step 3a: Implement the decorator**

Create `internal/api/breaker_publisher.go`:

```go
package api

import (
	"github.com/bikinitop/tracker/internal/circuitbreaker"
	"github.com/bikinitop/tracker/internal/tracker"
)

// breakerPublisher wraps an EventPublisher with a circuit breaker that
// fast-fails (ErrCircuitOpen) when NATS publishing is unhealthy.
type breakerPublisher struct {
	inner   EventPublisher
	breaker *circuitbreaker.Breaker
}

// NewBreakerPublisher returns an EventPublisher guarded by breaker.
func NewBreakerPublisher(inner EventPublisher, breaker *circuitbreaker.Breaker) EventPublisher {
	return &breakerPublisher{inner: inner, breaker: breaker}
}

func (p *breakerPublisher) PublishEvent(event *tracker.Event) error {
	if !p.breaker.Allow() {
		return circuitbreaker.ErrCircuitOpen
	}
	err := p.inner.PublishEvent(event)
	p.breaker.Record(err == nil)
	return err
}
```

- [ ] **Step 3b: Map ErrCircuitOpen to 503 in the handler**

In `internal/api/handler.go`, update the import block to add `"errors"` and the circuitbreaker package:

```go
import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/bikinitop/tracker/internal/circuitbreaker"
	"github.com/bikinitop/tracker/internal/tracker"
)
```

Then in `processSingleRequest`, replace this block:

```go
	if publisher != nil {
		if err := publisher.PublishEvent(event); err != nil {
			http.Error(w, "failed to publish event", http.StatusInternalServerError)
			return
		}
	}
```

with:

```go
	if publisher != nil {
		if err := publisher.PublishEvent(event); err != nil {
			if errors.Is(err, circuitbreaker.ErrCircuitOpen) {
				http.Error(w, "service unavailable", http.StatusServiceUnavailable)
				return
			}
			http.Error(w, "failed to publish event", http.StatusInternalServerError)
			return
		}
	}
```

(The bulk path's `publishEvent` helper is intentionally left unchanged: a circuit-open error there is recorded per-request in the bulk response's `errors` array, which is the correct behavior for a batch.)

- [ ] **Step 4: Run tests to verify they pass (race detector)**

Run: `go test -race ./internal/api/...`
Expected: PASS (all api tests, including the 3 new ones).

- [ ] **Step 5: Commit**

```bash
git add internal/api/breaker_publisher.go internal/api/breaker_publisher_test.go internal/api/handler.go
git commit -m "feat(api): circuit-breaker publisher decorator with 503 mapping"
```

---

## Task 8: Wire both features into the server

**Files:**
- Modify: `cmd/tracker/main.go`

- [ ] **Step 1: Update server construction and shutdown**

In `cmd/tracker/main.go`, add imports for the new packages and `golang.org/x/time/rate`:

```go
import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/time/rate"

	"github.com/bikinitop/tracker/internal/api"
	"github.com/bikinitop/tracker/internal/circuitbreaker"
	"github.com/bikinitop/tracker/internal/config"
	"github.com/bikinitop/tracker/internal/nats"
	"github.com/bikinitop/tracker/internal/ratelimit"
)
```

Add a `limiter` field to the `server` struct so it can be stopped on shutdown:

```go
type server struct {
	router    http.Handler
	addr      string
	connector natsConnector
	limiter   *ratelimit.IPRateLimiter
}
```

Replace the body of `newServer` with:

```go
func newServer(cfg *config.Config, connectFunc func(string) (natsConnector, error)) (*server, error) {
	var publisher api.EventPublisher
	var connector natsConnector
	if cfg.NATSURL != "" {
		var err error
		connector, err = connectFunc(cfg.NATSURL)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to NATS: %w", err)
		}
		publisher = nats.NewClientWrapper(connector)
	}

	// Guard publishing with a circuit breaker so a sick NATS fast-fails (503).
	if publisher != nil && cfg.CBEnabled {
		breaker := circuitbreaker.New(circuitbreaker.Config{
			FailureRatio:   cfg.CBFailureRatio,
			MinRequests:    cfg.CBMinRequests,
			Window:         cfg.CBWindow,
			OpenDuration:   cfg.CBOpenDuration,
			HalfOpenProbes: cfg.CBHalfOpenProbes,
		})
		publisher = api.NewBreakerPublisher(publisher, breaker)
	}

	// Per-IP rate limiting on /track (429).
	var limiter *ratelimit.IPRateLimiter
	var routerOpts []api.RouterOption
	if cfg.RateLimitEnabled {
		limiter = ratelimit.NewIPRateLimiter(rate.Limit(cfg.RateLimitRPS), cfg.RateLimitBurst, cfg.RateLimitIPTTL)
		routerOpts = append(routerOpts, api.WithRateLimiter(limiter, cfg.TrustProxy))
	}

	router := api.NewRouter(publisher, routerOpts...)
	addr := fmt.Sprintf(":%s", cfg.Port)

	return &server{router: router, addr: addr, connector: connector, limiter: limiter}, nil
}
```

In `runWithContext`, after the existing `if srv.connector != nil { defer srv.connector.Close() }`, add:

```go
	if srv.limiter != nil {
		defer srv.limiter.Stop()
	}
```

- [ ] **Step 2: Build and run the full suite with the race detector**

Run: `go build ./... && go test -race ./...`
Expected: build succeeds; all packages PASS.

- [ ] **Step 3: Tidy modules**

Run: `go mod tidy`
Expected: `golang.org/x/time` is now a direct (non-indirect) require; no other unexpected changes.

- [ ] **Step 4: Commit**

```bash
git add cmd/tracker/main.go go.mod go.sum
git commit -m "feat: wire rate limiting and circuit breaker into server"
```

---

## Task 9: Hot-path benchmark with protections enabled

**Files:**
- Create: `internal/api/protections_bench_test.go`

- [ ] **Step 1: Write the benchmark**

Create `internal/api/protections_bench_test.go`:

```go
package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bikinitop/tracker/internal/circuitbreaker"
	"github.com/bikinitop/tracker/internal/ratelimit"
	"golang.org/x/time/rate"
)

// BenchmarkTrack_WithProtections measures /track with both the rate limiter
// and circuit-breaker publisher in the chain. High limits keep both closed so
// we measure steady-state overhead, not rejection.
func BenchmarkTrack_WithProtections(b *testing.B) {
	limiter := ratelimit.NewIPRateLimiter(rate.Limit(1e9), 1e9, time.Minute)
	defer limiter.Stop()

	br := circuitbreaker.New(circuitbreaker.Config{
		FailureRatio:   0.5,
		MinRequests:    1 << 30,
		Window:         time.Hour,
		OpenDuration:   time.Second,
		HalfOpenProbes: 1,
	})
	publisher := NewBreakerPublisher(&noopPublisher{}, br)

	router := NewRouter(publisher, WithRateLimiter(limiter, false))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/track?idsite=1&rec=1", nil)
		req.RemoteAddr = "9.9.9.9:1234"
		router.ServeHTTP(httptest.NewRecorder(), req)
	}
}
```

- [ ] **Step 2: Run the benchmark**

Run: `go test ./internal/api/ -bench BenchmarkTrack_WithProtections -benchmem -run '^$'`
Expected: completes with a reported ns/op; compare informally against `BenchmarkTrackHandler` to confirm overhead is small (a few hundred ns at most).

- [ ] **Step 3: Commit**

```bash
git add internal/api/protections_bench_test.go
git commit -m "test(api): benchmark /track with rate limiter and breaker"
```

---

## Final verification

- [ ] **Run the full suite with the race detector**

Run: `go test -race ./...`
Expected: all packages PASS.

- [ ] **Confirm vet is clean**

Run: `go vet ./...`
Expected: no output.

---

## Self-review notes (spec coverage)

- Per-IP token bucket + 429 + Retry-After → Tasks 2, 5, 6.
- `/health` exempt → Task 6 (`TestRouter_HealthNotRateLimited`).
- Trust-proxy IP extraction → Task 5.
- `Limiter` interface seam + memory eviction → Task 2.
- Circuit breaker 3-state + injected clock + all transitions → Task 3.
- Decorator + `ErrCircuitOpen` → 503 → Task 7.
- Env-var config with fallback-on-error → Task 4.
- Wiring + toggles + clean shutdown → Task 8.
- Hot-path benchmark → Task 9.
