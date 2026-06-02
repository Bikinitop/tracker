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
