package nats

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/bikinitop/tracker/internal/tracker"
)

// MockPublisher is a test double for NATS publishing
type MockPublisher struct {
	Published [][]byte
	Subject   string
	Err       error
}

func (m *MockPublisher) Publish(subject string, data []byte) error {
	m.Subject = subject
	m.Published = append(m.Published, data)
	return m.Err
}

func TestPublisher_PublishEvent(t *testing.T) {
	mock := &MockPublisher{}
	publisher := NewPublisher(mock)

	event := &tracker.Event{
		SiteID:     "1",
		URL:        "https://example.com",
		ActionName: "Test",
	}

	err := publisher.PublishEvent(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mock.Subject != "tracker.1.pageview" {
		t.Errorf("expected subject tracker.1.pageview, got %s", mock.Subject)
	}

	if len(mock.Published) != 1 {
		t.Fatalf("expected 1 published message, got %d", len(mock.Published))
	}

	var published tracker.Event
	if err := json.Unmarshal(mock.Published[0], &published); err != nil {
		t.Fatalf("failed to unmarshal published event: %v", err)
	}

	if published.SiteID != "1" {
		t.Errorf("expected SiteID 1, got %s", published.SiteID)
	}
}

func TestPublisher_PublishEvent_ClientError(t *testing.T) {
	mock := &MockPublisher{Err: errors.New("nats connection failed")}
	publisher := NewPublisher(mock)

	event := &tracker.Event{
		SiteID: "1",
	}

	err := publisher.PublishEvent(event)
	if err == nil {
		t.Fatal("expected error for failed publish")
	}
}
