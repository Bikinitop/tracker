package config

import "os"

// Config holds application configuration
type Config struct {
	Port    string
	NATSURL string
}

// Load reads configuration from environment variables with defaults
func Load() *Config {
	return &Config{
		Port:    getEnv("TRACKER_PORT", "8080"),
		NATSURL: getEnv("NATS_URL", "nats://localhost:4222"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
