package nats

import (
	"encoding/json"
	"fmt"
	"strings"

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

// sanitizeSubjectToken replaces NATS wildcard characters that would break routing
func sanitizeSubjectToken(token string) string {
	// NATS wildcards: * matches any single token, > matches any remaining tokens
	// Dots are token separators — they create spurious hierarchy levels
	token = strings.ReplaceAll(token, ".", "_")
	token = strings.ReplaceAll(token, "*", "_")
	token = strings.ReplaceAll(token, ">", "_")
	return token
}

// PublishEvent serializes and publishes a tracking event to NATS
func (p *Publisher) PublishEvent(event *tracker.Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	subject := fmt.Sprintf("tracker.%s.%s",
		sanitizeSubjectToken(event.SiteID),
		sanitizeSubjectToken(event.ActionType))
	return p.client.Publish(subject, data)
}
