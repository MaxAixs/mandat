// Package config loads application configuration from a YAML file.
// Secrets (the database password) come only from the DB_PASSWORD environment
// variable and never from the config file.
// Section structs live next to their consumers: HTTP in transport/http,
// DB in pkg/database/postgres.
package config

import (
	"fmt"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/maxaixs/mandat/backend/internal/imgdownloader"
	"github.com/maxaixs/mandat/backend/internal/service"
	"github.com/maxaixs/mandat/backend/internal/storage/r2"
	"github.com/maxaixs/mandat/backend/internal/transport/http"
	"github.com/maxaixs/mandat/backend/pkg/database/postgres"
	"github.com/maxaixs/mandat/backend/pkg/redis"
	"github.com/spf13/viper"
)

const (
	passwordEnvVar      = "DB_PASSWORD"
	redisPasswordEnvVar = "REDIS_PASSWORD"
	r2AccessKeyEnvVar   = "R2_ACCESS_KEY_ID"
	r2SecretKeyEnvVar   = "R2_SECRET_ACCESS_KEY"
	r2AccountIDEnvVar   = "R2_ACCOUNT_ID"
	r2EndpointEnvVar    = "R2_ENDPOINT"
	r2BucketEnvVar      = "R2_BUCKET"
	r2PublicBaseEnvVar  = "R2_PUBLIC_BASE_URL"
	secureCookiesEnvVar = "HTTP_SECURE_COOKIES"
	allowedOriginsEnv   = "HTTP_ALLOWED_ORIGINS"

	// minRateLimitWindow is the shortest window that still makes sense for a limit.
	minRateLimitWindow = time.Second
	// minRatesRefreshInterval keeps the rate tickers from hammering the providers.
	minRatesRefreshInterval = time.Minute
)

// localhostName is the loopback hostname that may run without secure cookies.
const localhostName = "localhost"

// Config is the full application configuration.
type Config struct {
	LogLevel  string                   `mapstructure:"log_level"`
	HTTP      httptransport.HTTPConfig `mapstructure:"http"`
	DB        postgres.DBConfig        `mapstructure:"db"`
	Redis     redis.Config             `mapstructure:"redis"`
	Rates     RatesConfig              `mapstructure:"rates"`
	Storage   r2.Config                `mapstructure:"storage"`
	Download  imgdownloader.Config     `mapstructure:"download"`
	RateLimit service.RateLimitConfig  `mapstructure:"rate_limit"`
	// LoginRateLimit throttles login attempts per client IP and per account.
	LoginRateLimit service.RateLimitConfig `mapstructure:"login_rate_limit"`
}

// RatesConfig holds the upstream rate provider endpoints.
// URLs are endpoints without query parameters; the services add their own.
type RatesConfig struct {
	NBUURL  string `mapstructure:"nbu_url"`
	FuelURL string `mapstructure:"fuel_url"`
	// RefreshInterval is how often the rate tables are re-fetched.
	RefreshInterval time.Duration `mapstructure:"refresh_interval"`
	// HTTPTimeout bounds one request to a rate provider.
	HTTPTimeout time.Duration `mapstructure:"http_timeout"`
}

// Load reads the YAML file at path and validates it.
// The database password always comes from getenv("DB_PASSWORD").
func Load(path string, getenv func(string) string) (*Config, error) {
	v := viper.New()
	setDefaults(v)

	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config file %s: %w", path, err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("parse config file %s: %w", path, err)
	}

	cfg.DB.Password = getenv(passwordEnvVar)
	cfg.Redis.Password = getenv(redisPasswordEnvVar)
	cfg.Storage.AccessKeyID = getenv(r2AccessKeyEnvVar)
	cfg.Storage.SecretAccessKey = getenv(r2SecretKeyEnvVar)
	if value := getenv(r2EndpointEnvVar); value != "" {
		cfg.Storage.Endpoint = value
	}
	if value := getenv(r2BucketEnvVar); value != "" {
		cfg.Storage.Bucket = value
	}
	if value := getenv(r2PublicBaseEnvVar); value != "" {
		cfg.Storage.PublicBaseURL = value
	}
	if cfg.Storage.Endpoint == "" {
		if accountID := getenv(r2AccountIDEnvVar); accountID != "" {
			cfg.Storage.Endpoint = "https://" + accountID + ".r2.cloudflarestorage.com"
		}
	}
	if err := applyHTTPEnv(&cfg.HTTP, getenv); err != nil {
		return nil, err
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// setDefaults pins safe values for settings omitted from the file.
func setDefaults(v *viper.Viper) {
	v.SetDefault("log_level", "info")
	v.SetDefault("http.port", 8080)
	v.SetDefault("http.startup_timeout", 15*time.Second)
	v.SetDefault("http.read_header_timeout", 5*time.Second)
	v.SetDefault("http.read_timeout", 10*time.Second)
	v.SetDefault("http.write_timeout", 30*time.Second)
	v.SetDefault("http.idle_timeout", 60*time.Second)
	v.SetDefault("http.shutdown_timeout", 10*time.Second)
	v.SetDefault("http.secure_cookies", false)
	v.SetDefault("http.session_ttl", 12*time.Hour)
	v.SetDefault("db.port", 5432)
	v.SetDefault("db.sslmode", "disable")
	v.SetDefault("redis.addr", "localhost:6379")
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.dial_timeout", 5*time.Second)
	v.SetDefault("rates.nbu_url", "https://bank.gov.ua/NBU_Exchange/exchange")
	v.SetDefault("rates.fuel_url", "https://20.com.ua/api/v1/prices")
	v.SetDefault("rates.refresh_interval", 24*time.Hour)
	v.SetDefault("rates.http_timeout", 10*time.Second)
	v.SetDefault("download.timeout", 15*time.Second)
	v.SetDefault("download.max_bytes", 10<<20)
	v.SetDefault("download.max_redirects", 3)
	v.SetDefault("http.client_ip_headers", []string{"CF-Connecting-IP"})
	v.SetDefault("rate_limit.enabled", true)
	v.SetDefault("rate_limit.requests", 300)
	v.SetDefault("rate_limit.window", time.Minute)
	v.SetDefault("login_rate_limit.enabled", true)
	v.SetDefault("login_rate_limit.requests", 5)
	v.SetDefault("login_rate_limit.window", 5*time.Minute)
}

// applyHTTPEnv lets the deployment override the cookie and origin policy
// without rebuilding the image that ships config.yaml.
func applyHTTPEnv(cfg *httptransport.HTTPConfig, getenv func(string) string) error {
	if raw := getenv(secureCookiesEnvVar); raw != "" {
		secure, err := strconv.ParseBool(raw)
		if err != nil {
			return fmt.Errorf("%s must be a boolean, got %q", secureCookiesEnvVar, raw)
		}
		cfg.SecureCookies = secure
	}

	if raw := getenv(allowedOriginsEnv); raw != "" {
		var origins []string
		for origin := range strings.SplitSeq(raw, ",") {
			if origin = strings.TrimSpace(origin); origin != "" {
				origins = append(origins, origin)
			}
		}
		cfg.AllowedOrigins = origins
	}

	return nil
}

// validate fails fast on missing required settings.
func (c *Config) validate() error {
	switch {
	case c.DB.Host == "":
		return fmt.Errorf("db.host is required in the config file")
	case c.DB.User == "":
		return fmt.Errorf("db.user is required in the config file")
	case c.DB.Database == "":
		return fmt.Errorf("db.database is required in the config file")
	case c.DB.Password == "":
		return fmt.Errorf("%s environment variable is required (the password never lives in the config file)", passwordEnvVar)
	}
	if err := validateRateURL("rates.nbu_url", c.Rates.NBUURL); err != nil {
		return err
	}

	if err := validateRateURL("rates.fuel_url", c.Rates.FuelURL); err != nil {
		return err
	}

	if err := c.Rates.validateIntervals(); err != nil {
		return err
	}

	if err := c.validateHTTP(); err != nil {
		return err
	}

	if c.Storage.Configured() {
		if err := c.Storage.Validate(); err != nil {
			return err
		}
	}

	if err := validateTrustedProxies(c.HTTP.TrustedProxies); err != nil {
		return err
	}

	if err := validateRateLimit("rate_limit", c.RateLimit); err != nil {
		return err
	}

	return validateRateLimit("login_rate_limit", c.LoginRateLimit)
}

// validateHTTP checks the timeouts against the slowest request and the cookie policy
// against the origins: secure cookies may be off only for a purely local frontend.
func (c *Config) validateHTTP() error {
	if c.Download.Timeout >= c.HTTP.WriteTimeout {
		return fmt.Errorf("download.timeout (%s) must be shorter than http.write_timeout (%s)",
			c.Download.Timeout, c.HTTP.WriteTimeout)
	}

	for _, origin := range c.HTTP.AllowedOrigins {
		host, err := originHost(origin)
		if err != nil {
			return err
		}
		addr, err := netip.ParseAddr(host)
		loopback := strings.EqualFold(host, localhostName) || (err == nil && addr.IsLoopback())
		if !loopback && !c.HTTP.SecureCookies {
			return fmt.Errorf("http.secure_cookies must be true when http.allowed_origins includes %q", origin)
		}
	}

	return nil
}

// validateIntervals checks the rate refresh cadence and the provider timeout.
func (r RatesConfig) validateIntervals() error {
	if r.RefreshInterval < minRatesRefreshInterval {
		return fmt.Errorf("rates.refresh_interval must be at least %s", minRatesRefreshInterval)
	}

	if r.HTTPTimeout <= 0 {
		return fmt.Errorf("rates.http_timeout must be positive")
	}

	return nil
}

// originHost parses an allowed origin and returns its hostname.
func originHost(origin string) (string, error) {
	parsed, err := url.Parse(origin)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
		return "", fmt.Errorf("http.allowed_origins: %q must be an http(s) origin", origin)
	}

	return parsed.Hostname(), nil
}

// validateTrustedProxies accepts only IP addresses and CIDR ranges,
// so a typo cannot silently widen or drop the trusted proxy set.
func validateTrustedProxies(proxies []string) error {
	for _, proxy := range proxies {
		if _, err := netip.ParsePrefix(proxy); err == nil {
			continue
		}
		if _, err := netip.ParseAddr(proxy); err != nil {
			return fmt.Errorf("http.trusted_proxies: %q is neither an IP nor a CIDR", proxy)
		}
	}

	return nil
}

// validateRateLimit checks the limit tuning; a disabled limit is not validated.
func validateRateLimit(name string, cfg service.RateLimitConfig) error {
	if !cfg.Enabled {
		return nil
	}

	if cfg.Requests <= 0 {
		return fmt.Errorf("%s.requests must be positive", name)
	}

	if cfg.Window < minRateLimitWindow {
		return fmt.Errorf("%s.window must be at least %s", name, minRateLimitWindow)
	}

	return nil
}

// validateRateURL rejects blank URLs and non-HTTP schemes so a broken
// rates section fails fast instead of 503ing the ticker forever.
func validateRateURL(name, raw string) error {
	if strings.TrimSpace(raw) == "" {
		return fmt.Errorf("%s is required in the config file", name)
	}

	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("%s must be a valid http(s) URL, got %q", name, raw)
	}

	return nil
}
