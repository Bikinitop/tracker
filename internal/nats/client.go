package nats

import (
	"github.com/bikinitop/tracker/internal/tracker"
)

// ClientWrapper wraps a NATSClient to implement EventPublisher
type ClientWrapper struct {
	publisher *Publisher
}

// NewClientWrapper creates a new wrapper around a NATS client
func NewClientWrapper(client NATSClient) *ClientWrapper {
	return &ClientWrapper{
		publisher: NewPublisher(client),
	}
}

// PublishEvent delegates to the underlying publisher
func (c *ClientWrapper) PublishEvent(event *tracker.Event) error {
	return c.publisher.PublishEvent(event)
}
