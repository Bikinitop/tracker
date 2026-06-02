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

// getEnvParsed reads key and parses it with parse, falling back to defaultValue
// when the variable is unset or fails to parse.
func getEnvParsed[T any](key string, defaultValue T, parse func(string) (T, error)) T {
	if value, ok := os.LookupEnv(key); ok {
		if parsed, err := parse(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	return getEnvParsed(key, defaultValue, strconv.ParseBool)
}

func getEnvInt(key string, defaultValue int) int {
	return getEnvParsed(key, defaultValue, strconv.Atoi)
}

func getEnvFloat(key string, defaultValue float64) float64 {
	return getEnvParsed(key, defaultValue, func(s string) (float64, error) {
		return strconv.ParseFloat(s, 64)
	})
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	return getEnvParsed(key, defaultValue, time.ParseDuration)
}
