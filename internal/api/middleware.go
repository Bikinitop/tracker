package api

import (
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/bikinitop/tracker/internal/ratelimit"
)

// clientIP determines the client IP used as the rate-limit key. When trustProxy
// is true the rightmost X-Forwarded-For entry is used — the address appended by
// our own trusted proxy. Leftmost entries are client-supplied and forgeable, so
// keying on them would let a client bypass the per-IP limit by spoofing a fresh
// address per request. Otherwise the connection's RemoteAddr host is used.
func clientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			last := xff
			if i := strings.LastIndexByte(xff, ','); i >= 0 {
				last = xff[i+1:]
			}
			if ip := strings.TrimSpace(last); ip != "" {
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
