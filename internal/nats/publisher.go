package nats

import (
	"encoding/json"
	"fmt"

	"github.com/bikinitop/tracker/internal/tracker"
)

// NATSClient interface abstracts the NATS publish operation
type NATSClient interface {
	Publish(subject string, data []byte) error
}

// Publisher handles publishing tracking events to NATS
type Publisher struct {
	client NATSClient
}

// NewPublisher creates a new NATS event publisher
func NewPublisher(client NATSClient) *Publisher {
	return &Publisher{client: client}
}

// PublishEvent serializes and publishes a tracking event to NATS
func (p *Publisher) PublishEvent(event *tracker.Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	subject := fmt.Sprintf("tracker.%s.%s", event.SiteID, event.ActionType)
	return p.client.Publish(subject, data)
}
