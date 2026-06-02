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
