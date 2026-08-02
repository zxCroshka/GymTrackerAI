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
	defaultDBConnectTimeout      = 5 * time.Second
	defaultDBPingTimeout         = 2 * time.Second
	defaultDBMinConnections      = int32(1)
	defaultDBMaxConnections      = int32(10)
	defaultDBMaxConnLifetime     = 30 * time.Minute
	defaultDBMaxConnIdleTime     = 5 * time.Minute
	defaultDBHealthCheckPeriod   = time.Minute
)

// DatabaseConfig contains the PostgreSQL pool settings shared by API and
// database maintenance commands.
type DatabaseConfig struct {
	URL               string
	ConnectTimeout    time.Duration
	PingTimeout       time.Duration
	MinConnections    int32
	MaxConnections    int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
}

// Config contains foundation settings currently consumed by the API.
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
	Database              DatabaseConfig
}

// Load reads and validates configuration from environment variables.
func Load() (Config, error) {
	return load(os.Getenv)
}

// LoadDatabase reads only PostgreSQL settings for migration and seed commands.
func LoadDatabase() (DatabaseConfig, error) {
	return loadDatabase(os.Getenv)
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
	cfg.Database, err = loadDatabase(getenv)
	if err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func loadDatabase(getenv func(string) string) (DatabaseConfig, error) {
	databaseURL := strings.TrimSpace(getenv("DATABASE_URL"))
	if databaseURL == "" {
		return DatabaseConfig{}, fmt.Errorf("DATABASE_URL is required")
	}

	cfg := DatabaseConfig{URL: databaseURL}
	var err error
	cfg.ConnectTimeout, err = parsePositiveDuration("DB_CONNECT_TIMEOUT", getenv("DB_CONNECT_TIMEOUT"), defaultDBConnectTimeout)
	if err != nil {
		return DatabaseConfig{}, err
	}
	cfg.PingTimeout, err = parsePositiveDuration("DB_PING_TIMEOUT", getenv("DB_PING_TIMEOUT"), defaultDBPingTimeout)
	if err != nil {
		return DatabaseConfig{}, err
	}
	cfg.MinConnections, err = parseConnectionCount("DB_MIN_CONNS", getenv("DB_MIN_CONNS"), defaultDBMinConnections, true)
	if err != nil {
		return DatabaseConfig{}, err
	}
	cfg.MaxConnections, err = parseConnectionCount("DB_MAX_CONNS", getenv("DB_MAX_CONNS"), defaultDBMaxConnections, false)
	if err != nil {
		return DatabaseConfig{}, err
	}
	if cfg.MinConnections > cfg.MaxConnections {
		return DatabaseConfig{}, fmt.Errorf("DB_MIN_CONNS must not exceed DB_MAX_CONNS")
	}
	cfg.MaxConnLifetime, err = parsePositiveDuration("DB_MAX_CONN_LIFETIME", getenv("DB_MAX_CONN_LIFETIME"), defaultDBMaxConnLifetime)
	if err != nil {
		return DatabaseConfig{}, err
	}
	cfg.MaxConnIdleTime, err = parsePositiveDuration("DB_MAX_CONN_IDLE_TIME", getenv("DB_MAX_CONN_IDLE_TIME"), defaultDBMaxConnIdleTime)
	if err != nil {
		return DatabaseConfig{}, err
	}
	cfg.HealthCheckPeriod, err = parsePositiveDuration("DB_HEALTH_CHECK_PERIOD", getenv("DB_HEALTH_CHECK_PERIOD"), defaultDBHealthCheckPeriod)
	if err != nil {
		return DatabaseConfig{}, err
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

func parseConnectionCount(name, value string, fallback int32, allowZero bool) (int32, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}

	count, err := strconv.ParseInt(value, 10, 32)
	if err != nil || count < 0 || (!allowZero && count == 0) {
		if allowZero {
			return 0, fmt.Errorf("%s must be a non-negative integer", name)
		}
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return int32(count), nil
}
