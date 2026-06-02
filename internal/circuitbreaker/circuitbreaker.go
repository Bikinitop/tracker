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
	Window         time.Duration // tumbling window; failure/success counts reset once it elapses
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

// Record feeds the outcome of an allowed request back to the breaker. It must
// be called at most once per Allow() that returned true; calling it otherwise
// (e.g. while the breaker is half-open without a matching probe) may distort
// the breaker's accounting.
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
		if total > 0 && total >= b.cfg.MinRequests {
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
