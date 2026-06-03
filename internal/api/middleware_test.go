package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// stubLimiter returns a fixed verdict and records the last key it saw.
type stubLimiter struct {
	allow   bool
	lastKey string
}

func (s *stubLimiter) Allow(key string) bool {
	s.lastKey = key
	return s.allow
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("passed"))
	})
}

func TestRateLimitMiddleware_AllowsWhenUnderLimit(t *testing.T) {
	mw := RateLimitMiddleware(&stubLimiter{allow: true}, false, 1, okHandler())

	req := httptest.NewRequest(http.MethodGet, "/track?idsite=1&rec=1", nil)
	req.RemoteAddr = "9.9.9.9:1234"
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if rr.Body.String() != "passed" {
		t.Errorf("expected request to reach next handler")
	}
}

func TestRateLimitMiddleware_Returns429WhenOverLimit(t *testing.T) {
	mw := RateLimitMiddleware(&stubLimiter{allow: false}, false, 3, okHandler())

	req := httptest.NewRequest(http.MethodGet, "/track?idsite=1&rec=1", nil)
	req.RemoteAddr = "9.9.9.9:1234"
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rr.Code)
	}
	if got := rr.Header().Get("Retry-After"); got != "3" {
		t.Errorf("expected Retry-After 3, got %q", got)
	}
}

func TestRateLimitMiddleware_UsesRemoteAddrByDefault(t *testing.T) {
	stub := &stubLimiter{allow: true}
	mw := RateLimitMiddleware(stub, false, 1, okHandler())

	req := httptest.NewRequest(http.MethodGet, "/track?idsite=1&rec=1", nil)
	req.RemoteAddr = "5.6.7.8:4321"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	if stub.lastKey != "5.6.7.8" {
		t.Errorf("expected RemoteAddr host as key, got %q", stub.lastKey)
	}
}

func TestRateLimitMiddleware_UsesRightmostXForwardedForWhenTrusted(t *testing.T) {
	stub := &stubLimiter{allow: true}
	mw := RateLimitMiddleware(stub, true, 1, okHandler())

	req := httptest.NewRequest(http.MethodGet, "/track?idsite=1&rec=1", nil)
	req.RemoteAddr = "5.6.7.8:4321"
	// Leftmost (1.2.3.4) is client-forgeable; the rightmost (9.9.9.9) is the
	// address our trusted proxy appended and is what we must key on.
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 9.9.9.9")
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	if stub.lastKey != "9.9.9.9" {
		t.Errorf("expected rightmost XFF as key, got %q", stub.lastKey)
	}
}

func TestClientIP_EdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		xff        string
		trustProxy bool
		want       string
	}{
		{"remote addr with port", "1.2.3.4:5678", "", false, "1.2.3.4"},
		{"remote addr without port falls back to raw", "1.2.3.4", "", false, "1.2.3.4"},
		{"ipv6 remote addr", "[::1]:1234", "", false, "::1"},
		{"trusted but empty xff falls back to remote addr", "5.6.7.8:4321", "", true, "5.6.7.8"},
		{"trusted whitespace-only xff falls back to remote addr", "5.6.7.8:4321", "   ", true, "5.6.7.8"},
		{"trusted single-entry xff", "5.6.7.8:4321", "9.9.9.9", true, "9.9.9.9"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/track", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			if got := clientIP(req, tt.trustProxy); got != tt.want {
				t.Errorf("clientIP(%q, xff=%q, trust=%v) = %q, want %q",
					tt.remoteAddr, tt.xff, tt.trustProxy, got, tt.want)
			}
		})
	}
}
