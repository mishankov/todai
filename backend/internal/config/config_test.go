package config

import "testing"

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
}

func TestLoadReadsEnvironment(t *testing.T) {
	t.Parallel()

	values := map[string]string{
		"TODAI_DATABASE_URL":        "postgres://example",
		"TODAI_HTTP_PORT":           "9090",
		"TODAI_SESSION_COOKIE_NAME": "custom_session",
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
