package nats

import (
	"testing"

	"github.com/bikinitop/tracker/internal/tracker"
)

func TestClientWrapper_PublishEvent(t *testing.T) {
	mock := &MockPublisher{}
	wrapper := NewClientWrapper(mock)

	event := &tracker.Event{
		SiteID: "1",
		URL:    "https://example.com",
	}

	err := wrapper.PublishEvent(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.Published) != 1 {
		t.Fatalf("expected 1 published message, got %d", len(mock.Published))
	}
}
