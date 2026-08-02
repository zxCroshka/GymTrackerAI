package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadUsesFoundationDefaults(t *testing.T) {
	cfg, err := load(func(string) string { return "" })
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
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := load(func(key string) string {
				if key == test.key {
					return test.value
				}
				return ""
			})
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
