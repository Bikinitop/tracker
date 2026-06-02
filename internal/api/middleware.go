package api

import (
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/bikinitop/tracker/internal/ratelimit"
)

// clientIP determines the client IP used as the rate-limit key. When trustProxy
// is true the leftmost X-Forwarded-For entry is used (only safe behind a
// trusted proxy that sets it); otherwise the connection's RemoteAddr host.
func clientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if ip := strings.TrimSpace(strings.Split(xff, ",")[0]); ip != "" {
				return ip
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// RateLimitMiddleware rejects requests whose client IP has exhausted its quota
// with 429 and a Retry-After header; otherwise it calls next.
func RateLimitMiddleware(limiter ratelimit.Limiter, trustProxy bool, retryAfterSeconds int, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow(clientIP(r, trustProxy)) {
			w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds))
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
