package nats

import (
	"testing"

	"github.com/bikinitop/tracker/internal/tracker"
)

func BenchmarkPublisher_PublishEvent(b *testing.B) {
	mock := &MockPublisher{}
	publisher := NewPublisher(mock)

	event := &tracker.Event{
		SiteID:     "1",
		URL:        "https://example.com",
		ActionName: "Test",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := publisher.PublishEvent(event); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPublisher_PublishEvent_Parallel(b *testing.B) {
	mock := &MockPublisher{}
	publisher := NewPublisher(mock)

	event := &tracker.Event{
		SiteID:     "1",
		URL:        "https://example.com",
		ActionName: "Test",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := publisher.PublishEvent(event); err != nil {
				b.Fatal(err)
			}
		}
	})
}
