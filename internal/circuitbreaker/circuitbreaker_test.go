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

func TestBreaker_ZeroMinRequestsDoesNotTrip(t *testing.T) {
	cfg := testConfig()
	cfg.MinRequests = 0
	b := New(cfg)
	b.Record(true)
	if b.State() != StateClosed {
		t.Errorf("expected Closed with MinRequests=0, got %v", b.State())
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

func TestBreaker_FailuresExpireAfterWindow(t *testing.T) {
	current := time.Unix(1000, 0)
	b := New(testConfig(), WithClock(func() time.Time { return current }))

	// 3 failures: below MinRequests(4), stays Closed.
	for i := 0; i < 3; i++ {
		b.Record(false)
	}
	if b.State() != StateClosed {
		t.Fatalf("precondition: expected Closed below MinRequests")
	}

	// Advance past the window so the prior failures expire.
	current = current.Add(11 * time.Second)

	// 3 more failures. If the window had NOT reset, total would be 6 and the
	// breaker would have tripped on the 4th. With the reset it stays Closed.
	for i := 0; i < 3; i++ {
		b.Record(false)
	}
	if b.State() != StateClosed {
		t.Errorf("expected Closed after window reset cleared old failures, got %v", b.State())
	}
}

func TestBreaker_HalfOpenLimitsConcurrentProbes(t *testing.T) {
	current := time.Unix(1000, 0)
	b := New(testConfig(), WithClock(func() time.Time { return current }))
	for i := 0; i < 4; i++ {
		b.Record(false)
	}
	current = current.Add(5 * time.Second)

	// HalfOpenProbes = 2: with no Record between calls, only 2 probes are
	// admitted; the third is rejected until a probe reports back.
	if !b.Allow() {
		t.Errorf("probe 1 should be admitted")
	}
	if !b.Allow() {
		t.Errorf("probe 2 should be admitted")
	}
	if b.Allow() {
		t.Errorf("probe 3 should be rejected (inflight cap reached)")
	}
}

func TestBreaker_IgnoresRecordWhileOpen(t *testing.T) {
	current := time.Unix(1000, 0)
	b := New(testConfig(), WithClock(func() time.Time { return current }))
	for i := 0; i < 4; i++ {
		b.Record(false)
	}
	if b.State() != StateOpen {
		t.Fatalf("precondition: expected Open")
	}

	// Records while fully open must not change state; recovery is probe-driven.
	for i := 0; i < 10; i++ {
		b.Record(true)
	}
	if b.State() != StateOpen {
		t.Errorf("expected Open to ignore records, got %v", b.State())
	}
	if b.Allow() {
		t.Errorf("expected Allow to stay false before OpenDuration elapses")
	}
}

func TestNew_ClampsInvalidConfig(t *testing.T) {
	// A zero Config would otherwise trip on the first success (FailureRatio 0
	// means 0 >= 0). Clamping to safe defaults must prevent that.
	b := New(Config{})
	b.Record(true)
	if b.State() != StateClosed {
		t.Errorf("expected clamped breaker to stay Closed on success, got %v", b.State())
	}
}

func TestState_String(t *testing.T) {
	cases := map[State]string{
		StateClosed:   "closed",
		StateOpen:     "open",
		StateHalfOpen: "half-open",
		State(99):     "unknown",
	}
	for s, want := range cases {
		if got := s.String(); got != want {
			t.Errorf("State(%d).String() = %q, want %q", int(s), got, want)
		}
	}
}
