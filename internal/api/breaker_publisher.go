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
