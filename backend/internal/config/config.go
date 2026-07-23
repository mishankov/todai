// Package config loads and validates backend configuration.
package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	defaultEnvironment   = "development"
	defaultDatabaseURL   = "postgres://todai:todai@localhost:5432/todai?sslmode=disable"
	defaultHTTPPort      = "8080"
	defaultCookieName    = "todai_session"
	defaultLogFormat     = "text"
	defaultRunnerExec    = "node"
	defaultRunnerEntry   = "../pi-runner/dist/cli/main.js"
	defaultRunnerLine    = 1024 * 1024
	defaultAgentRuntime  = "fake"
	defaultAgentTokenTTL = 15 * time.Minute
)

// Config contains the process configuration required by the backend.
type Config struct {
	Environment       string
	DatabaseURL       string
	HTTPPort          string
	SessionCookieName string
	LogFormat         string
	RunnerExecutable  string
	RunnerEntry       string
	RunnerStartup     time.Duration
	RunnerRunTimeout  time.Duration
	RunnerAbort       time.Duration
	RunnerMaximumLine int
	AgentRuntime      string
	InternalAPIURL    string
	AgentTokenTTL     time.Duration
	PiAgentDirectory  string
	PiProvider        string
	PiModel           string
	PiModels          []string
}

// Load reads configuration from the process environment.
func Load() (Config, error) {
	return load(os.Getenv)
}

func load(getenv func(string) string) (Config, error) {
	cfg := Config{
		Environment:       valueOrDefault(getenv("TODAI_ENVIRONMENT"), defaultEnvironment),
		DatabaseURL:       valueOrDefault(getenv("TODAI_DATABASE_URL"), defaultDatabaseURL),
		HTTPPort:          valueOrDefault(getenv("TODAI_HTTP_PORT"), defaultHTTPPort),
		SessionCookieName: valueOrDefault(getenv("TODAI_SESSION_COOKIE_NAME"), defaultCookieName),
		LogFormat:         valueOrDefault(getenv("TODAI_LOG_FORMAT"), defaultLogFormat),
		RunnerExecutable:  valueOrDefault(getenv("TODAI_RUNNER_EXECUTABLE"), defaultRunnerExec),
		RunnerEntry:       valueOrDefault(getenv("TODAI_RUNNER_ENTRY"), defaultRunnerEntry),
		AgentRuntime:      valueOrDefault(getenv("TODAI_AGENT_RUNTIME"), defaultAgentRuntime),
		PiAgentDirectory:  strings.TrimSpace(getenv("TODAI_PI_AGENT_DIR")),
		PiProvider:        strings.TrimSpace(getenv("TODAI_PI_PROVIDER")),
		PiModel:           strings.TrimSpace(getenv("TODAI_PI_MODEL")),
		PiModels:          commaSeparatedValues(getenv("TODAI_PI_MODELS")),
	}
	if cfg.Environment != "development" && cfg.Environment != "production" {
		return Config{}, errors.New("TODAI_ENVIRONMENT must be development or production")
	}
	if cfg.Environment == "production" && getenv("TODAI_LOG_FORMAT") == "" {
		cfg.LogFormat = "json"
	}
	if cfg.LogFormat != "json" && cfg.LogFormat != "text" {
		return Config{}, errors.New("TODAI_LOG_FORMAT must be json or text")
	}
	if cfg.Environment == "production" {
		if err := validateProductionDatabaseURL(getenv("TODAI_DATABASE_URL")); err != nil {
			return Config{}, err
		}
	}
	if len(cfg.PiModels) == 0 && cfg.PiModel != "" {
		cfg.PiModels = []string{cfg.PiModel}
	}
	if cfg.PiModel != "" && !contains(cfg.PiModels, cfg.PiModel) {
		return Config{}, errors.New("TODAI_PI_MODEL must be included in TODAI_PI_MODELS")
	}

	if _, err := net.LookupPort("tcp", cfg.HTTPPort); err != nil {
		return Config{}, fmt.Errorf("invalid TODAI_HTTP_PORT %q: %w", cfg.HTTPPort, err)
	}
	var err error
	if cfg.RunnerStartup, err = durationValue(
		getenv("TODAI_RUNNER_STARTUP_TIMEOUT"), 5*time.Second,
	); err != nil {
		return Config{}, fmt.Errorf("invalid TODAI_RUNNER_STARTUP_TIMEOUT: %w", err)
	}
	if cfg.RunnerRunTimeout, err = durationValue(
		getenv("TODAI_RUNNER_RUN_TIMEOUT"), 2*time.Minute,
	); err != nil {
		return Config{}, fmt.Errorf("invalid TODAI_RUNNER_RUN_TIMEOUT: %w", err)
	}
	if cfg.RunnerAbort, err = durationValue(
		getenv("TODAI_RUNNER_ABORT_TIMEOUT"), 2*time.Second,
	); err != nil {
		return Config{}, fmt.Errorf("invalid TODAI_RUNNER_ABORT_TIMEOUT: %w", err)
	}
	if cfg.RunnerMaximumLine, err = positiveIntegerValue(
		getenv("TODAI_RUNNER_MAX_LINE_BYTES"), defaultRunnerLine,
	); err != nil {
		return Config{}, fmt.Errorf("invalid TODAI_RUNNER_MAX_LINE_BYTES: %w", err)
	}
	if cfg.AgentTokenTTL, err = durationValue(
		getenv("TODAI_AGENT_TOKEN_TTL"), defaultAgentTokenTTL,
	); err != nil || cfg.AgentTokenTTL > 15*time.Minute {
		return Config{}, errors.New("invalid TODAI_AGENT_TOKEN_TTL: must be between 1ns and 15m")
	}
	if cfg.AgentTokenTTL < cfg.RunnerRunTimeout {
		return Config{}, errors.New("TODAI_AGENT_TOKEN_TTL must be at least TODAI_RUNNER_RUN_TIMEOUT")
	}
	if cfg.AgentRuntime != "fake" && cfg.AgentRuntime != "pi" {
		return Config{}, errors.New("TODAI_AGENT_RUNTIME must be fake or pi")
	}
	if cfg.Environment == "production" && cfg.AgentRuntime == "pi" {
		if !filepath.IsAbs(cfg.PiAgentDirectory) {
			return Config{}, errors.New("TODAI_PI_AGENT_DIR must be an absolute path when TODAI_AGENT_RUNTIME=pi")
		}
		if cfg.PiProvider == "" || cfg.PiModel == "" {
			return Config{}, errors.New("TODAI_PI_PROVIDER and TODAI_PI_MODEL are required when TODAI_AGENT_RUNTIME=pi")
		}
	}
	cfg.InternalAPIURL = strings.TrimRight(valueOrDefault(
		getenv("TODAI_INTERNAL_API_URL"), "http://127.0.0.1:"+cfg.HTTPPort,
	), "/")
	parsedInternalURL, parseErr := url.Parse(cfg.InternalAPIURL)
	if parseErr != nil || (parsedInternalURL.Scheme != "http" && parsedInternalURL.Scheme != "https") || parsedInternalURL.Host == "" {
		return Config{}, errors.New("invalid TODAI_INTERNAL_API_URL: must be an absolute HTTP(S) URL")
	}
	if parsedInternalURL.User != nil || parsedInternalURL.RawQuery != "" || parsedInternalURL.Fragment != "" {
		return Config{}, errors.New("invalid TODAI_INTERNAL_API_URL: credentials, query, and fragment are not allowed")
	}

	return cfg, nil
}

func validateProductionDatabaseURL(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return errors.New("TODAI_DATABASE_URL is required in production")
	}
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") ||
		parsed.Hostname() == "" || parsed.User == nil || strings.Trim(parsed.Path, "/") == "" {
		return errors.New("invalid production TODAI_DATABASE_URL: expected a PostgreSQL URL with host, database, and credentials")
	}
	username := parsed.User.Username()
	password, hasPassword := parsed.User.Password()
	if username == "" || !hasPassword || password == "" {
		return errors.New("invalid production TODAI_DATABASE_URL: username and password are required")
	}
	if username == "todai" || password == "todai" {
		return errors.New("development PostgreSQL credentials are not allowed in production")
	}
	return nil
}

func valueOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}

	return value
}

func durationValue(value string, fallback time.Duration) (time.Duration, error) {
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return 0, errors.New("must be a positive duration")
	}
	return parsed, nil
}

func positiveIntegerValue(value string, fallback int) (int, error) {
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, errors.New("must be a positive integer")
	}
	return parsed, nil
}

func commaSeparatedValues(value string) []string {
	values := make([]string, 0)
	seen := make(map[string]struct{})
	for item := range strings.SplitSeq(value, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		values = append(values, item)
	}
	return values
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
