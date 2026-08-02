package config

import (
	"fmt"
	"net"
	"net/url"
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
	defaultJWTIssuer             = "gymtracker-api"
	defaultJWTAccessAudience     = "gymtracker-web"
	defaultJWTRefreshAudience    = "gymtracker-refresh"
	defaultAccessTokenTTL        = 15 * time.Minute
	defaultRefreshTokenTTL       = 30 * 24 * time.Hour
	defaultRefreshCookieName     = "gymtracker_refresh"
	defaultAuthRateLimit         = 10
	defaultAuthRateWindow        = time.Minute
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

// AuthConfig contains JWT, cookie, origin, and endpoint-throttling settings.
type AuthConfig struct {
	AccessSecret    string
	RefreshSecret   string
	Issuer          string
	AccessAudience  string
	RefreshAudience string
	AccessTTL       time.Duration
	RefreshTTL      time.Duration
	CookieName      string
	CookieSecure    bool
	AllowedOrigins  []string
	RateLimit       int
	RateWindow      time.Duration
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
	Auth                  AuthConfig
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
	cfg.Auth, err = loadAuth(getenv, cfg.Environment)
	if err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func loadAuth(getenv func(string) string, environment string) (AuthConfig, error) {
	cfg := AuthConfig{
		AccessSecret:    strings.TrimSpace(getenv("JWT_ACCESS_SECRET")),
		RefreshSecret:   strings.TrimSpace(getenv("JWT_REFRESH_SECRET")),
		Issuer:          valueOrDefault(getenv("JWT_ISSUER"), defaultJWTIssuer),
		AccessAudience:  valueOrDefault(getenv("JWT_ACCESS_AUDIENCE"), defaultJWTAccessAudience),
		RefreshAudience: valueOrDefault(getenv("JWT_REFRESH_AUDIENCE"), defaultJWTRefreshAudience),
		CookieName:      valueOrDefault(getenv("REFRESH_COOKIE_NAME"), defaultRefreshCookieName),
	}
	if len(cfg.AccessSecret) < 32 {
		return AuthConfig{}, fmt.Errorf("JWT_ACCESS_SECRET must contain at least 32 characters")
	}
	if len(cfg.RefreshSecret) < 32 {
		return AuthConfig{}, fmt.Errorf("JWT_REFRESH_SECRET must contain at least 32 characters")
	}
	if cfg.AccessSecret == cfg.RefreshSecret {
		return AuthConfig{}, fmt.Errorf("JWT access and refresh secrets must differ")
	}

	var err error
	cfg.AccessTTL, err = parsePositiveDuration("JWT_ACCESS_TTL", getenv("JWT_ACCESS_TTL"), defaultAccessTokenTTL)
	if err != nil {
		return AuthConfig{}, err
	}
	cfg.RefreshTTL, err = parsePositiveDuration("JWT_REFRESH_TTL", getenv("JWT_REFRESH_TTL"), defaultRefreshTokenTTL)
	if err != nil {
		return AuthConfig{}, err
	}
	if cfg.AccessTTL > 30*time.Minute {
		return AuthConfig{}, fmt.Errorf("JWT_ACCESS_TTL must not exceed 30 minutes")
	}
	if cfg.RefreshTTL > 90*24*time.Hour {
		return AuthConfig{}, fmt.Errorf("JWT_REFRESH_TTL must not exceed 90 days")
	}
	if cfg.RefreshTTL <= cfg.AccessTTL {
		return AuthConfig{}, fmt.Errorf("JWT_REFRESH_TTL must exceed JWT_ACCESS_TTL")
	}
	cfg.RateWindow, err = parsePositiveDuration("AUTH_RATE_WINDOW", getenv("AUTH_RATE_WINDOW"), defaultAuthRateWindow)
	if err != nil {
		return AuthConfig{}, err
	}
	cfg.RateLimit, err = parsePositiveInt("AUTH_RATE_LIMIT", getenv("AUTH_RATE_LIMIT"), defaultAuthRateLimit)
	if err != nil {
		return AuthConfig{}, err
	}
	cfg.CookieSecure, err = parseBool("REFRESH_COOKIE_SECURE", getenv("REFRESH_COOKIE_SECURE"), environment == "production")
	if err != nil {
		return AuthConfig{}, err
	}
	if environment == "production" && !cfg.CookieSecure {
		return AuthConfig{}, fmt.Errorf("REFRESH_COOKIE_SECURE must be true in production")
	}

	for _, value := range strings.Split(getenv("AUTH_ALLOWED_ORIGINS"), ",") {
		if origin := strings.TrimSpace(value); origin != "" {
			normalized, err := validateOrigin(origin, environment)
			if err != nil {
				return AuthConfig{}, err
			}
			cfg.AllowedOrigins = append(cfg.AllowedOrigins, normalized)
		}
	}
	if len(cfg.AllowedOrigins) == 0 {
		if environment == "production" {
			return AuthConfig{}, fmt.Errorf("AUTH_ALLOWED_ORIGINS is required in production")
		}
		cfg.AllowedOrigins = []string{"http://localhost:3000"}
	}
	return cfg, nil
}

func validateOrigin(value, environment string) (string, error) {
	parsed, err := url.Parse(strings.TrimSuffix(value, "/"))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil ||
		parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return "", fmt.Errorf("AUTH_ALLOWED_ORIGINS must contain only absolute origins")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("AUTH_ALLOWED_ORIGINS supports only http and https")
	}
	if environment == "production" && parsed.Scheme != "https" {
		return "", fmt.Errorf("AUTH_ALLOWED_ORIGINS must use https in production")
	}
	return parsed.Scheme + "://" + parsed.Host, nil
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

func parsePositiveInt(name, value string, fallback int) (int, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return parsed, nil
}

func parseBool(name, value string, fallback bool) (bool, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false", name)
	}
	return parsed, nil
}
