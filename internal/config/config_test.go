package config

import (
	"os"
	"testing"
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
