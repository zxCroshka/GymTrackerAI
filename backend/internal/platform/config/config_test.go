package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadUsesFoundationDefaults(t *testing.T) {
	cfg, err := load(testEnvironment(map[string]string{}))
	if err != nil {
		t.Fatalf("load defaults: %v", err)
	}

	if cfg.AppName != defaultAppName {
		t.Fatalf("AppName = %q, want %q", cfg.AppName, defaultAppName)
	}
	if cfg.Environment != defaultEnvironment {
		t.Fatalf("Environment = %q, want %q", cfg.Environment, defaultEnvironment)
	}
	if cfg.HTTPAddress() != "0.0.0.0:8080" {
		t.Fatalf("HTTPAddress = %q, want %q", cfg.HTTPAddress(), "0.0.0.0:8080")
	}
	if cfg.HTTPShutdownTimeout != defaultHTTPShutdownTimeout {
		t.Fatalf("HTTPShutdownTimeout = %v, want %v", cfg.HTTPShutdownTimeout, defaultHTTPShutdownTimeout)
	}
	if cfg.Database.MinConnections != 1 || cfg.Database.MaxConnections != 10 {
		t.Fatalf("unexpected database pool defaults: %+v", cfg.Database)
	}
}

func TestLoadReadsCustomValues(t *testing.T) {
	values := map[string]string{
		"APP_NAME":                 "gymtracker-test",
		"APP_ENV":                  "test",
		"LOG_LEVEL":                "DEBUG",
		"HTTP_HOST":                "127.0.0.1",
		"HTTP_PORT":                "9090",
		"HTTP_READ_TIMEOUT":        "2s",
		"HTTP_READ_HEADER_TIMEOUT": "1s",
		"HTTP_WRITE_TIMEOUT":       "3s",
		"HTTP_IDLE_TIMEOUT":        "4s",
		"HTTP_SHUTDOWN_TIMEOUT":    "5s",
		"DATABASE_URL":             "postgres://database.example/gymtracker",
		"DB_CONNECT_TIMEOUT":       "6s",
		"DB_PING_TIMEOUT":          "7s",
		"DB_MIN_CONNS":             "2",
		"DB_MAX_CONNS":             "12",
		"DB_MAX_CONN_LIFETIME":     "20m",
		"DB_MAX_CONN_IDLE_TIME":    "3m",
		"DB_HEALTH_CHECK_PERIOD":   "30s",
	}

	cfg, err := load(func(key string) string { return values[key] })
	if err != nil {
		t.Fatalf("load custom values: %v", err)
	}

	if cfg.LogLevel != "debug" || cfg.HTTPAddress() != "127.0.0.1:9090" {
		t.Fatalf("unexpected custom config: %+v", cfg)
	}
	if cfg.HTTPReadTimeout != 2*time.Second || cfg.HTTPShutdownTimeout != 5*time.Second {
		t.Fatalf("unexpected custom durations: %+v", cfg)
	}
	if cfg.Database.ConnectTimeout != 6*time.Second || cfg.Database.MinConnections != 2 || cfg.Database.MaxConnections != 12 {
		t.Fatalf("unexpected custom database config: %+v", cfg.Database)
	}
}

func TestLoadRejectsInvalidValuesWithoutEchoingThem(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		value     string
		wantError string
	}{
		{name: "environment", key: "APP_ENV", value: "private-value", wantError: "APP_ENV"},
		{name: "log level", key: "LOG_LEVEL", value: "private-value", wantError: "LOG_LEVEL"},
		{name: "port", key: "HTTP_PORT", value: "private-value", wantError: "HTTP_PORT"},
		{name: "duration", key: "HTTP_IDLE_TIMEOUT", value: "private-value", wantError: "HTTP_IDLE_TIMEOUT"},
		{name: "database duration", key: "DB_PING_TIMEOUT", value: "private-value", wantError: "DB_PING_TIMEOUT"},
		{name: "database connections", key: "DB_MAX_CONNS", value: "private-value", wantError: "DB_MAX_CONNS"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := load(testEnvironment(map[string]string{
				test.key: test.value,
			}))
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("error %q does not mention %q", err, test.wantError)
			}
			if strings.Contains(err.Error(), test.value) {
				t.Fatalf("error %q echoes rejected value", err)
			}
		})
	}
}

func TestLoadDatabaseValidatesRequiredURLAndPoolBounds(t *testing.T) {
	if _, err := loadDatabase(func(string) string { return "" }); err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("missing URL error = %v", err)
	}

	_, err := loadDatabase(testEnvironment(map[string]string{
		"DB_MIN_CONNS": "11",
		"DB_MAX_CONNS": "10",
	}))
	if err == nil || !strings.Contains(err.Error(), "DB_MIN_CONNS") {
		t.Fatalf("pool bounds error = %v", err)
	}
}

func testEnvironment(values map[string]string) func(string) string {
	return func(key string) string {
		if key == "DATABASE_URL" {
			if value, ok := values[key]; ok {
				return value
			}
			return "postgres://database.example/gymtracker"
		}
		return values[key]
	}
}
