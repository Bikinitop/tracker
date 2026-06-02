package api

import (
	"net/http"

	"github.com/bikinitop/tracker/internal/ratelimit"
)

type routerConfig struct {
	limiter    ratelimit.Limiter
	trustProxy bool
}

// RouterOption configures the router.
type RouterOption func(*routerConfig)

// WithRateLimiter wraps the /track endpoint in per-IP rate limiting.
func WithRateLimiter(limiter ratelimit.Limiter, trustProxy bool) RouterOption {
	return func(rc *routerConfig) {
		rc.limiter = limiter
		rc.trustProxy = trustProxy
	}
}

// NewRouter creates an HTTP router with all tracking endpoints. Options may
// enable rate limiting on the tracking endpoint.
func NewRouter(publisher EventPublisher, opts ...RouterOption) http.Handler {
	rc := &routerConfig{}
	for _, opt := range opts {
		opt(rc)
	}

	mux := http.NewServeMux()

	// Bikinitop branded tracking endpoint
	var trackHandler http.Handler = TrackHandler(publisher)
	if rc.limiter != nil {
		trackHandler = RateLimitMiddleware(rc.limiter, rc.trustProxy, 1, trackHandler)
	}
	mux.Handle("/track", trackHandler)

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	return mux
}
