// Package config loads application configuration from a YAML file.
// Secrets (the database password) come only from the DB_PASSWORD environment
// variable and never from the config file.
// Section structs live next to their consumers: HTTP in transport/http,
// DB in pkg/database/postgres.
package config

import (
	"fmt"
	"time"

	"github.com/maxaixs/mandat/backend/internal/transport/http"
	"github.com/maxaixs/mandat/backend/pkg/database/postgres"
	"github.com/maxaixs/mandat/backend/pkg/redis"
	"github.com/spf13/viper"
)

const (
	passwordEnvVar      = "DB_PASSWORD"
	redisPasswordEnvVar = "REDIS_PASSWORD"
)

// Config is the full application configuration.
type Config struct {
	LogLevel string                   `mapstructure:"log_level"`
	HTTP     httptransport.HTTPConfig `mapstructure:"http"`
	DB       postgres.DBConfig        `mapstructure:"db"`
	Redis    redis.Config             `mapstructure:"redis"`
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
	v.SetDefault("http.write_timeout", 10*time.Second)
	v.SetDefault("http.idle_timeout", 60*time.Second)
	v.SetDefault("http.shutdown_timeout", 10*time.Second)
	v.SetDefault("http.secure_cookies", false)
	v.SetDefault("http.session_ttl", 12*time.Hour)
	v.SetDefault("db.port", 5432)
	v.SetDefault("db.sslmode", "disable")
	v.SetDefault("redis.addr", "localhost:6379")
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.dial_timeout", 5*time.Second)
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

	return nil
}
