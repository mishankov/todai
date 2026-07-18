package config

import (
	"testing"
	"time"
)

func TestLoadUsesDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := load(func(string) string { return "" })
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.DatabaseURL != defaultDatabaseURL {
		t.Errorf("DatabaseURL = %q, want %q", cfg.DatabaseURL, defaultDatabaseURL)
	}
	if cfg.HTTPPort != defaultHTTPPort {
		t.Errorf("HTTPPort = %q, want %q", cfg.HTTPPort, defaultHTTPPort)
	}
	if cfg.SessionCookieName != defaultCookieName {
		t.Errorf("SessionCookieName = %q, want %q", cfg.SessionCookieName, defaultCookieName)
	}
	if cfg.RunnerExecutable != defaultRunnerExec || cfg.RunnerEntry != defaultRunnerEntry {
		t.Errorf("runner = (%q, %q)", cfg.RunnerExecutable, cfg.RunnerEntry)
	}
	if cfg.RunnerStartup != 5*time.Second || cfg.RunnerRunTimeout != 2*time.Minute ||
		cfg.RunnerAbort != 2*time.Second || cfg.RunnerMaximumLine != defaultRunnerLine {
		t.Errorf("runner limits = %#v", cfg)
	}
	if cfg.AgentRuntime != "fake" || cfg.InternalAPIURL != "http://127.0.0.1:8080" ||
		cfg.AgentTokenTTL != 15*time.Minute {
		t.Errorf("agent configuration = %#v", cfg)
	}
}

func TestLoadReadsEnvironment(t *testing.T) {
	t.Parallel()

	values := map[string]string{
		"TODAI_DATABASE_URL":           "postgres://example",
		"TODAI_HTTP_PORT":              "9090",
		"TODAI_SESSION_COOKIE_NAME":    "custom_session",
		"TODAI_RUNNER_EXECUTABLE":      "/usr/local/bin/node",
		"TODAI_RUNNER_ENTRY":           "/srv/todai/runner.js",
		"TODAI_RUNNER_STARTUP_TIMEOUT": "7s",
		"TODAI_RUNNER_RUN_TIMEOUT":     "3m",
		"TODAI_RUNNER_ABORT_TIMEOUT":   "4s",
		"TODAI_RUNNER_MAX_LINE_BYTES":  "2048",
		"TODAI_AGENT_RUNTIME":          "pi",
		"TODAI_INTERNAL_API_URL":       "https://tools.example.test/base/",
		"TODAI_AGENT_TOKEN_TTL":        "10m",
		"TODAI_PI_AGENT_DIR":           "/srv/pi",
		"TODAI_PI_PROVIDER":            "openai-codex",
		"TODAI_PI_MODEL":               "gpt-5.6-sol",
		"TODAI_PI_MODELS":              "gpt-5.6-sol, gpt-5.6-terra, gpt-5.6-sol",
	}

	cfg, err := load(func(key string) string { return values[key] })
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.DatabaseURL != values["TODAI_DATABASE_URL"] {
		t.Errorf("DatabaseURL = %q", cfg.DatabaseURL)
	}
	if cfg.HTTPPort != values["TODAI_HTTP_PORT"] {
		t.Errorf("HTTPPort = %q", cfg.HTTPPort)
	}
	if cfg.SessionCookieName != values["TODAI_SESSION_COOKIE_NAME"] {
		t.Errorf("SessionCookieName = %q", cfg.SessionCookieName)
	}
	if cfg.RunnerExecutable != values["TODAI_RUNNER_EXECUTABLE"] ||
		cfg.RunnerEntry != values["TODAI_RUNNER_ENTRY"] || cfg.RunnerStartup != 7*time.Second ||
		cfg.RunnerRunTimeout != 3*time.Minute || cfg.RunnerAbort != 4*time.Second ||
		cfg.RunnerMaximumLine != 2048 {
		t.Errorf("runner configuration = %#v", cfg)
	}
	if cfg.AgentRuntime != "pi" || cfg.InternalAPIURL != "https://tools.example.test/base" ||
		cfg.AgentTokenTTL != 10*time.Minute || cfg.PiAgentDirectory != "/srv/pi" ||
		cfg.PiProvider != "openai-codex" || cfg.PiModel != "gpt-5.6-sol" ||
		len(cfg.PiModels) != 2 || cfg.PiModels[1] != "gpt-5.6-terra" {
		t.Errorf("agent configuration = %#v", cfg)
	}
}

func TestLoadRejectsInvalidAgentConfiguration(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name  string
		key   string
		value string
	}{
		{name: "runtime", key: "TODAI_AGENT_RUNTIME", value: "unknown"},
		{name: "URL", key: "TODAI_INTERNAL_API_URL", value: "file:///tmp/tools"},
		{name: "TTL too short", key: "TODAI_AGENT_TOKEN_TTL", value: "1m"},
		{name: "TTL too long", key: "TODAI_AGENT_TOKEN_TTL", value: "16m"},
		{name: "default model is not allowed", key: "TODAI_PI_MODELS", value: "other-model"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := load(func(key string) string {
				if key == test.key {
					return test.value
				}
				if test.name == "default model is not allowed" && key == "TODAI_PI_MODEL" {
					return "default-model"
				}
				return ""
			})
			if err == nil {
				t.Fatal("load config succeeded with invalid agent configuration")
			}
		})
	}
}

func TestLoadRejectsInvalidPort(t *testing.T) {
	t.Parallel()

	_, err := load(func(key string) string {
		if key == "TODAI_HTTP_PORT" {
			return "not-a-port"
		}
		return ""
	})
	if err == nil {
		t.Fatal("load config succeeded with an invalid port")
	}
}

func TestLoadRejectsInvalidRunnerLimits(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name  string
		key   string
		value string
	}{
		{name: "startup", key: "TODAI_RUNNER_STARTUP_TIMEOUT", value: "never"},
		{name: "run", key: "TODAI_RUNNER_RUN_TIMEOUT", value: "0s"},
		{name: "abort", key: "TODAI_RUNNER_ABORT_TIMEOUT", value: "-1s"},
		{name: "line", key: "TODAI_RUNNER_MAX_LINE_BYTES", value: "0"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := load(func(key string) string {
				if key == test.key {
					return test.value
				}
				return ""
			})
			if err == nil {
				t.Fatal("load config succeeded with an invalid runner limit")
			}
		})
	}
}
