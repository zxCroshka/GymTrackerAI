package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAppName               = "gymtracker-backend"
	defaultEnvironment           = "development"
	defaultLogLevel              = "info"
	defaultHTTPHost              = "0.0.0.0"
	defaultHTTPPort              = 8080
	defaultHTTPReadTimeout       = 10 * time.Second
	defaultHTTPReadHeaderTimeout = 5 * time.Second
	defaultHTTPWriteTimeout      = 15 * time.Second
	defaultHTTPIdleTimeout       = 60 * time.Second
	defaultHTTPShutdownTimeout   = 10 * time.Second
)

// Config contains only foundation settings currently consumed by the API.
type Config struct {
	AppName               string
	Environment           string
	LogLevel              string
	HTTPHost              string
	HTTPPort              int
	HTTPReadTimeout       time.Duration
	HTTPReadHeaderTimeout time.Duration
	HTTPWriteTimeout      time.Duration
	HTTPIdleTimeout       time.Duration
	HTTPShutdownTimeout   time.Duration
}

// Load reads and validates configuration from environment variables.
func Load() (Config, error) {
	return load(os.Getenv)
}

func load(getenv func(string) string) (Config, error) {
	cfg := Config{
		AppName:     valueOrDefault(getenv("APP_NAME"), defaultAppName),
		Environment: valueOrDefault(getenv("APP_ENV"), defaultEnvironment),
		LogLevel:    strings.ToLower(valueOrDefault(getenv("LOG_LEVEL"), defaultLogLevel)),
		HTTPHost:    valueOrDefault(getenv("HTTP_HOST"), defaultHTTPHost),
	}

	if err := validateEnvironment(cfg.Environment); err != nil {
		return Config{}, err
	}
	if err := validateLogLevel(cfg.LogLevel); err != nil {
		return Config{}, err
	}

	var err error
	cfg.HTTPPort, err = parsePort(getenv("HTTP_PORT"))
	if err != nil {
		return Config{}, err
	}
	cfg.HTTPReadTimeout, err = parsePositiveDuration("HTTP_READ_TIMEOUT", getenv("HTTP_READ_TIMEOUT"), defaultHTTPReadTimeout)
	if err != nil {
		return Config{}, err
	}
	cfg.HTTPReadHeaderTimeout, err = parsePositiveDuration("HTTP_READ_HEADER_TIMEOUT", getenv("HTTP_READ_HEADER_TIMEOUT"), defaultHTTPReadHeaderTimeout)
	if err != nil {
		return Config{}, err
	}
	cfg.HTTPWriteTimeout, err = parsePositiveDuration("HTTP_WRITE_TIMEOUT", getenv("HTTP_WRITE_TIMEOUT"), defaultHTTPWriteTimeout)
	if err != nil {
		return Config{}, err
	}
	cfg.HTTPIdleTimeout, err = parsePositiveDuration("HTTP_IDLE_TIMEOUT", getenv("HTTP_IDLE_TIMEOUT"), defaultHTTPIdleTimeout)
	if err != nil {
		return Config{}, err
	}
	cfg.HTTPShutdownTimeout, err = parsePositiveDuration("HTTP_SHUTDOWN_TIMEOUT", getenv("HTTP_SHUTDOWN_TIMEOUT"), defaultHTTPShutdownTimeout)
	if err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// HTTPAddress returns a net/http-compatible listen address.
func (c Config) HTTPAddress() string {
	return net.JoinHostPort(c.HTTPHost, strconv.Itoa(c.HTTPPort))
}

func valueOrDefault(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func validateEnvironment(value string) error {
	switch value {
	case "development", "test", "production":
		return nil
	default:
		return fmt.Errorf("APP_ENV must be development, test, or production")
	}
}

func validateLogLevel(value string) error {
	switch value {
	case "debug", "info", "warn", "error":
		return nil
	default:
		return fmt.Errorf("LOG_LEVEL must be debug, info, warn, or error")
	}
}

func parsePort(value string) (int, error) {
	if strings.TrimSpace(value) == "" {
		return defaultHTTPPort, nil
	}

	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("HTTP_PORT must be an integer from 1 through 65535")
	}
	return port, nil
}

func parsePositiveDuration(name, value string, fallback time.Duration) (time.Duration, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}

	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("%s must be a positive Go duration", name)
	}
	return duration, nil
}
