package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/maxaixs/mandat/backend/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fullYAML = `
log_level: debug
http:
  port: 9000
  allowed_origins:
    - http://localhost:5173
    - https://mandat.example
  startup_timeout: 20s
  read_header_timeout: 7s
  read_timeout: 11s
  write_timeout: 12s
  idle_timeout: 100s
  shutdown_timeout: 9s
db:
  host: db-host
  port: 5433
  user: dbuser
  database: mandadb
  sslmode: require
`

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func envMap(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

// Representative spot-check that mapstructure tags match the YAML keys.
func TestLoad_ParsesAllSections(t *testing.T) {
	cfg, err := config.Load(writeConfig(t, fullYAML), envMap(map[string]string{"DB_PASSWORD": "secret"}))

	require.NoError(t, err)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, 9000, cfg.HTTP.Port)
	assert.Equal(t, 20*time.Second, cfg.HTTP.StartupTimeout)
	assert.Equal(t, "db-host", cfg.DB.Host)
	assert.Equal(t, "require", cfg.DB.SSLMode)
	assert.Equal(t, []string{"http://localhost:5173", "https://mandat.example"}, cfg.HTTP.AllowedOrigins)
}

func TestLoad_DefaultsForMissingValues(t *testing.T) {
	const minimalYAML = "db:\n  host: localhost\n  user: mandat\n  database: mandat\n"

	cfg, err := config.Load(writeConfig(t, minimalYAML), envMap(map[string]string{"DB_PASSWORD": "secret"}))

	require.NoError(t, err)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, 8080, cfg.HTTP.Port)
	assert.Equal(t, 15*time.Second, cfg.HTTP.StartupTimeout)
	assert.Equal(t, 5*time.Second, cfg.HTTP.ReadHeaderTimeout)
	assert.Equal(t, 10*time.Second, cfg.HTTP.ReadTimeout)
	assert.Equal(t, 10*time.Second, cfg.HTTP.WriteTimeout)
	assert.Equal(t, 60*time.Second, cfg.HTTP.IdleTimeout)
	assert.Equal(t, 10*time.Second, cfg.HTTP.ShutdownTimeout)
	assert.Equal(t, 5432, cfg.DB.Port)
	assert.Equal(t, "disable", cfg.DB.SSLMode)
	assert.Empty(t, cfg.HTTP.AllowedOrigins)
}

func TestLoad_PasswordOnlyFromEnv(t *testing.T) {
	t.Setenv("DB_PASSWORD", "from-env")

	withPassword := strings.Replace(fullYAML, "  sslmode: require\n", "  password: from-yaml\n  sslmode: disable\n", 1)
	cfg, err := config.Load(writeConfig(t, withPassword), os.Getenv)

	require.NoError(t, err)
	assert.Equal(t, "from-env", cfg.DB.Password)
}

func TestLoad_MissingPasswordFails(t *testing.T) {
	cfg, err := config.Load(writeConfig(t, fullYAML), envMap(nil))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "DB_PASSWORD")
	assert.Nil(t, cfg)
}
