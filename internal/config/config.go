package config

import "os"

// Config holds application configuration
type Config struct {
	Port    string
	NATSURL string
}

// Load reads configuration from environment variables with defaults
func Load() *Config {
	cfg := &Config{
		Port:    getEnv("TRACKER_PORT", "8080"),
		NATSURL: getEnv("NATS_URL", "nats://localhost:4222"),
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
