// Package config loads and validates backend configuration.
package config

import (
	"fmt"
	"net"
	"os"
)

const (
	defaultDatabaseURL = "postgres://todai:todai@localhost:5432/todai?sslmode=disable"
	defaultHTTPPort    = "8080"
	defaultCookieName  = "todai_session"
)

// Config contains the process configuration required by the backend.
type Config struct {
	DatabaseURL       string
	HTTPPort          string
	SessionCookieName string
}

// Load reads configuration from the process environment.
func Load() (Config, error) {
	return load(os.Getenv)
}

func load(getenv func(string) string) (Config, error) {
	cfg := Config{
		DatabaseURL:       valueOrDefault(getenv("TODAI_DATABASE_URL"), defaultDatabaseURL),
		HTTPPort:          valueOrDefault(getenv("TODAI_HTTP_PORT"), defaultHTTPPort),
		SessionCookieName: valueOrDefault(getenv("TODAI_SESSION_COOKIE_NAME"), defaultCookieName),
	}

	if _, err := net.LookupPort("tcp", cfg.HTTPPort); err != nil {
		return Config{}, fmt.Errorf("invalid TODAI_HTTP_PORT %q: %w", cfg.HTTPPort, err)
	}

	return cfg, nil
}

func valueOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}

	return value
}
