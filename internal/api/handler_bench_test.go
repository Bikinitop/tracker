package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bikinitop/tracker/internal/tracker"
)

type noopPublisher struct{}

func (n *noopPublisher) PublishEvent(event *tracker.Event) error { return nil }

func BenchmarkTrackHandler(b *testing.B) {
	handler := TrackHandler(&noopPublisher{})

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/matomo.php?idsite=1&rec=1&url=https%3A%2F%2Fexample.com", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}
}

func BenchmarkTrackHandler_Parallel(b *testing.B) {
	handler := TrackHandler(&noopPublisher{})

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/matomo.php?idsite=1&rec=1&url=https%3A%2F%2Fexample.com", nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
		}
	})
}
