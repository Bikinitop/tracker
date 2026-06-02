package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds application configuration.
type Config struct {
	Port    string
	NATSURL string

	// Per-IP rate limiting
	RateLimitEnabled bool
	RateLimitRPS     float64
	RateLimitBurst   int
	RateLimitIPTTL   time.Duration
	TrustProxy       bool

	// NATS-publish circuit breaker
	CBEnabled        bool
	CBFailureRatio   float64
	CBMinRequests    int
	CBWindow         time.Duration
	CBOpenDuration   time.Duration
	CBHalfOpenProbes int
}

// Load reads configuration from environment variables with defaults.
func Load() *Config {
	cfg := &Config{
		Port:    getEnv("TRACKER_PORT", "8080"),
		NATSURL: getEnv("NATS_URL", "nats://localhost:4222"),

		RateLimitEnabled: getEnvBool("TRACKER_RATELIMIT_ENABLED", true),
		RateLimitRPS:     getEnvFloat("TRACKER_RATELIMIT_RPS", 50),
		RateLimitBurst:   getEnvInt("TRACKER_RATELIMIT_BURST", 100),
		RateLimitIPTTL:   getEnvDuration("TRACKER_RATELIMIT_IP_TTL", 10*time.Minute),
		TrustProxy:       getEnvBool("TRACKER_TRUST_PROXY", false),

		CBEnabled:        getEnvBool("TRACKER_CB_ENABLED", true),
		CBFailureRatio:   getEnvFloat("TRACKER_CB_FAILURE_RATIO", 0.5),
		CBMinRequests:    getEnvInt("TRACKER_CB_MIN_REQUESTS", 20),
		CBWindow:         getEnvDuration("TRACKER_CB_WINDOW", 10*time.Second),
		CBOpenDuration:   getEnvDuration("TRACKER_CB_OPEN_DURATION", 5*time.Second),
		CBHalfOpenProbes: getEnvInt("TRACKER_CB_HALF_OPEN_PROBES", 5),
	}

	// Allow explicitly disabling NATS by setting NATS_URL=disabled or empty
	if cfg.NATSURL == "disabled" {
		cfg.NATSURL = ""
	}

	return cfg
}

func getEnv(key, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(value); err == nil {
			return n
		}
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value, ok := os.LookupEnv(key); ok {
		if f, err := strconv.ParseFloat(value, 64); err == nil {
			return f
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return defaultValue
}
