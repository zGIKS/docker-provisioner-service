package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	withIsolatedEnv(t, func() {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.Port != "3000" {
			t.Fatalf("port = %q, want 3000", cfg.Port)
		}
		if cfg.HTTPBodyLimitBytes != 1024*1024 {
			t.Fatalf("body limit = %d, want %d", cfg.HTTPBodyLimitBytes, 1024*1024)
		}
		if cfg.HTTPReadTimeout != 10*time.Second {
			t.Fatalf("read timeout = %s, want 10s", cfg.HTTPReadTimeout)
		}
		if cfg.RateLimitMax != 60 {
			t.Fatalf("rate limit max = %d, want 60", cfg.RateLimitMax)
		}
	})
}

func TestLoadInvalidEnv(t *testing.T) {
	withIsolatedEnv(t, func() {
		t.Setenv("HTTP_BODY_LIMIT_BYTES", "invalid")
		_, err := Load()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func withIsolatedEnv(t *testing.T, fn func()) {
	t.Helper()
	keys := []string{
		"PORT",
		"DOCKER_BIN",
		"DOCKER_COMMAND_TIMEOUT_SECONDS",
		"TENANT_DB_IMAGE",
		"TENANT_DB_NETWORK",
		"TENANT_DB_HOST",
		"TENANT_DB_USER",
		"TENANT_DB_NAME_PREFIX",
		"TENANT_DB_MEMORY_MB",
		"TENANT_DB_CPU_CORES",
		"HTTP_BODY_LIMIT_BYTES",
		"HTTP_READ_TIMEOUT_SECONDS",
		"HTTP_WRITE_TIMEOUT_SECONDS",
		"HTTP_IDLE_TIMEOUT_SECONDS",
		"HTTP_RATE_LIMIT_MAX",
		"HTTP_RATE_LIMIT_WINDOW_SECONDS",
	}

	backup := make(map[string]*string, len(keys))
	for _, k := range keys {
		v, ok := os.LookupEnv(k)
		if ok {
			vv := v
			backup[k] = &vv
		} else {
			backup[k] = nil
		}
		_ = os.Unsetenv(k)
	}
	t.Cleanup(func() {
		for _, k := range keys {
			if backup[k] == nil {
				_ = os.Unsetenv(k)
			} else {
				_ = os.Setenv(k, *backup[k])
			}
		}
	})

	fn()
}
