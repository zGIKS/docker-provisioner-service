package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port                 string
	DockerBin            string
	DockerCommandTimeout time.Duration
	TenantDBImage        string
	TenantDBNetwork      string
	TenantDBHost         string
	TenantDBUser         string
	TenantDBNamePrefix   string
	DefaultMemoryMB      *int64
	DefaultCPUCores      *float64
}

func Load() (Config, error) {
	cfg := Config{
		Port:               getEnv("PORT", "3000"),
		DockerBin:          getEnv("DOCKER_BIN", "docker"),
		TenantDBImage:      getEnv("TENANT_DB_IMAGE", "postgres:16-alpine"),
		TenantDBNetwork:    getEnv("TENANT_DB_NETWORK", "auth-tenants"),
		TenantDBHost:       getEnv("TENANT_DB_HOST", "127.0.0.1"),
		TenantDBUser:       getEnv("TENANT_DB_USER", "tenant_user"),
		TenantDBNamePrefix: getEnv("TENANT_DB_NAME_PREFIX", "tenant_"),
	}

	timeoutSec, err := parseInt64Env("DOCKER_COMMAND_TIMEOUT_SECONDS")
	if err != nil {
		return cfg, err
	}
	if timeoutSec == nil {
		defaultTimeout := int64(120)
		timeoutSec = &defaultTimeout
	}
	cfg.DockerCommandTimeout = time.Duration(*timeoutSec) * time.Second

	cfg.DefaultMemoryMB, err = parseInt64Env("TENANT_DB_MEMORY_MB")
	if err != nil {
		return cfg, err
	}

	cfg.DefaultCPUCores, err = parseFloat64Env("TENANT_DB_CPU_CORES")
	if err != nil {
		return cfg, err
	}

	return cfg, nil
}

func parseInt64Env(key string) (*int64, error) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("%s must be int64", key)
	}
	return &parsed, nil
}

func parseFloat64Env(key string) (*float64, error) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return nil, fmt.Errorf("%s must be float64", key)
	}
	return &parsed, nil
}

func getEnv(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}
