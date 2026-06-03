package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	cfg := Load()

	if cfg.Port != "8080" {
		t.Errorf("expected default port 8080, got %s", cfg.Port)
	}
	if cfg.NATSURL != "nats://localhost:4222" {
		t.Errorf("expected default NATS URL nats://localhost:4222, got %s", cfg.NATSURL)
	}
}

func TestLoad_FromEnv(t *testing.T) {
	os.Setenv("TRACKER_PORT", "9090")
	os.Setenv("NATS_URL", "nats://custom:4222")
	defer os.Unsetenv("TRACKER_PORT")
	defer os.Unsetenv("NATS_URL")

	cfg := Load()

	if cfg.Port != "9090" {
		t.Errorf("expected port 9090, got %s", cfg.Port)
	}
	if cfg.NATSURL != "nats://custom:4222" {
		t.Errorf("expected NATS URL nats://custom:4222, got %s", cfg.NATSURL)
	}
}

func TestLoad_NATS_Disabled(t *testing.T) {
	os.Setenv("NATS_URL", "disabled")
	defer os.Unsetenv("NATS_URL")

	cfg := Load()

	if cfg.NATSURL != "" {
		t.Errorf("expected empty NATS URL when disabled, got %s", cfg.NATSURL)
	}
}

func TestLoad_RateLimitDefaults(t *testing.T) {
	cfg := Load()
	if !cfg.RateLimitEnabled {
		t.Errorf("expected rate limiting enabled by default")
	}
	if cfg.RateLimitRPS != 50 {
		t.Errorf("expected default RPS 50, got %v", cfg.RateLimitRPS)
	}
	if cfg.RateLimitBurst != 100 {
		t.Errorf("expected default burst 100, got %d", cfg.RateLimitBurst)
	}
	if cfg.RateLimitIPTTL != 10*time.Minute {
		t.Errorf("expected default TTL 10m, got %v", cfg.RateLimitIPTTL)
	}
	if cfg.TrustProxy {
		t.Errorf("expected TrustProxy false by default")
	}
}

func TestLoad_CircuitBreakerDefaults(t *testing.T) {
	cfg := Load()
	if !cfg.CBEnabled {
		t.Errorf("expected circuit breaker enabled by default")
	}
	if cfg.CBFailureRatio != 0.5 {
		t.Errorf("expected default failure ratio 0.5, got %v", cfg.CBFailureRatio)
	}
	if cfg.CBMinRequests != 20 {
		t.Errorf("expected default min requests 20, got %d", cfg.CBMinRequests)
	}
	if cfg.CBWindow != 10*time.Second {
		t.Errorf("expected default window 10s, got %v", cfg.CBWindow)
	}
	if cfg.CBOpenDuration != 5*time.Second {
		t.Errorf("expected default open duration 5s, got %v", cfg.CBOpenDuration)
	}
	if cfg.CBHalfOpenProbes != 5 {
		t.Errorf("expected default half-open probes 5, got %d", cfg.CBHalfOpenProbes)
	}
}

func TestLoad_RateLimitFromEnv(t *testing.T) {
	os.Setenv("TRACKER_RATELIMIT_ENABLED", "false")
	os.Setenv("TRACKER_RATELIMIT_RPS", "100")
	os.Setenv("TRACKER_TRUST_PROXY", "true")
	defer os.Unsetenv("TRACKER_RATELIMIT_ENABLED")
	defer os.Unsetenv("TRACKER_RATELIMIT_RPS")
	defer os.Unsetenv("TRACKER_TRUST_PROXY")

	cfg := Load()
	if cfg.RateLimitEnabled {
		t.Errorf("expected rate limiting disabled")
	}
	if cfg.RateLimitRPS != 100 {
		t.Errorf("expected RPS 100, got %v", cfg.RateLimitRPS)
	}
	if !cfg.TrustProxy {
		t.Errorf("expected TrustProxy true")
	}
}

func TestLoad_InvalidValuesFallBackToDefaults(t *testing.T) {
	os.Setenv("TRACKER_RATELIMIT_RPS", "not-a-number")
	os.Setenv("TRACKER_CB_WINDOW", "garbage")
	defer os.Unsetenv("TRACKER_RATELIMIT_RPS")
	defer os.Unsetenv("TRACKER_CB_WINDOW")

	cfg := Load()
	if cfg.RateLimitRPS != 50 {
		t.Errorf("expected fallback RPS 50, got %v", cfg.RateLimitRPS)
	}
	if cfg.CBWindow != 10*time.Second {
		t.Errorf("expected fallback window 10s, got %v", cfg.CBWindow)
	}
}
