package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bikinitop/tracker/internal/circuitbreaker"
	"github.com/bikinitop/tracker/internal/ratelimit"
	"golang.org/x/time/rate"
)

// BenchmarkTrack_WithProtections measures /track with both the rate limiter
// and circuit-breaker publisher in the chain. High limits keep both closed so
// we measure steady-state overhead, not rejection.
func BenchmarkTrack_WithProtections(b *testing.B) {
	limiter := ratelimit.NewIPRateLimiter(rate.Limit(1e9), 1e9, time.Minute)
	defer limiter.Stop()

	br := circuitbreaker.New(circuitbreaker.Config{
		FailureRatio:   0.5,
		MinRequests:    1 << 30,
		Window:         time.Hour,
		OpenDuration:   time.Second,
		HalfOpenProbes: 1,
	})
	publisher := NewBreakerPublisher(&noopPublisher{}, br)

	router := NewRouter(publisher, WithRateLimiter(limiter, false))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/track?idsite=1&rec=1", nil)
		req.RemoteAddr = "9.9.9.9:1234"
		router.ServeHTTP(httptest.NewRecorder(), req)
	}
}
