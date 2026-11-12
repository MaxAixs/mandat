package config_test

import (
	"os"
	"path"
	"path/filepath"
	"runtime"
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

// shippedConfigPath resolves the repository's single config file from this
// test file's location, so the assertions do not depend on the working directory.
func shippedConfigPath(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok, "resolve config test file location")

	return filepath.Join(filepath.Dir(file), "..", "..", "config.yaml")
}

// Guards the single shipped config against drifting back to localhost-only
// values that cannot reach the Docker Compose services by name.
func TestShippedConfig_UsesComposeServiceNames(t *testing.T) {
	cfg, err := config.Load(shippedConfigPath(t), envMap(map[string]string{"DB_PASSWORD": "secret"}))

	require.NoError(t, err)
	assert.Equal(t, "db", cfg.DB.Host)
	assert.Equal(t, "redis:6379", cfg.Redis.Addr)
	assert.Equal(t, "mandat", cfg.DB.Database)
	assert.Equal(t, "mandat", cfg.DB.User)
	assert.Equal(t, 0, cfg.Redis.DB)
	assert.Equal(t, 5*time.Second, cfg.Redis.DialTimeout)
	assert.Equal(t, 8080, cfg.HTTP.Port)

	// A mistyped key (e.g. "nbu-url:") silently falls back to the defaults, so
	// pin both the keys and the values actually shipped in config.yaml.
	assert.Equal(t, "https://bank.gov.ua/NBU_Exchange/exchange", cfg.Rates.NBUURL,
		"config.yaml must set rates.nbu_url to the NBU exchange endpoint")
	assert.Equal(t, "https://20.com.ua/api/v1/prices", cfg.Rates.FuelURL,
		"config.yaml must set rates.fuel_url to the 20.com.ua prices endpoint")

	content, err := os.ReadFile(shippedConfigPath(t))
	require.NoError(t, err)
	assert.Contains(t, string(content), "nbu_url:",
		"config.yaml must spell the rates key nbu_url, not a variant")
	assert.Contains(t, string(content), "fuel_url:",
		"config.yaml must spell the rates key fuel_url, not a variant")
}

// The shipped config must stay secret-free: without DB_PASSWORD the service
// cannot start, and the file itself must not carry a password key.
func TestShippedConfig_HasNoCredentials(t *testing.T) {
	configPath := shippedConfigPath(t)

	cfg, err := config.Load(configPath, envMap(nil))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "DB_PASSWORD")
	assert.Nil(t, cfg)

	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	// Key-level check: an indentation-trimmed YAML mapping key reads "<key>:",
	// which catches nested keys such as db.password while ignoring prose in
	// comments and values.
	for i, line := range strings.Split(string(content), "\n") {
		key := strings.ToLower(strings.TrimSpace(line))
		assert.False(
			t,
			strings.HasPrefix(key, "password:"),
			"config.yaml line %d must not define a password key: %s",
			i+1,
			line,
		)
	}
}

// dockerfileCopySources extracts the local COPY sources (the last argument is
// the destination). Build-stage copies and glob/. sources are not from the
// build context and are skipped.
func dockerfileCopySources(t *testing.T, dockerfile string) []string {
	t.Helper()

	var sources []string

	for _, line := range strings.Split(dockerfile, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 3 || !strings.EqualFold(fields[0], "COPY") {
			continue
		}

		args := fields[1:]
		fromStage := false

		for len(args) > 0 && strings.HasPrefix(args[0], "--") {
			if strings.HasPrefix(args[0], "--from=") {
				fromStage = true
			}

			args = args[1:]
		}

		if fromStage || len(args) < 2 {
			continue
		}

		for _, src := range args[:len(args)-1] {
			if src == "." || strings.ContainsAny(src, "*?[") {
				continue
			}

			sources = append(sources, src)
		}
	}

	return sources
}

// dockerignored reports whether a single .dockerignore pattern excludes the
// given build-context-relative source path. Patterns are rooted at the build
// context, so a leading slash is insignificant; a trailing slash only means
// directory. A pattern also ignores everything below a directory it matches.
func dockerignored(pattern, src string) bool {
	pattern = strings.TrimSuffix(strings.TrimPrefix(pattern, "/"), "/")
	src = strings.TrimPrefix(src, "/")

	if pattern == "" || src == "" {
		return false
	}

	if matched, err := path.Match(pattern, src); err == nil && matched {
		return true
	}

	// Walk the parent directories so "docs" and "doc*" both exclude "docs/x".
	for dir := path.Dir(src); dir != "." && dir != "/"; dir = path.Dir(dir) {
		if matched, err := path.Match(pattern, dir); err == nil && matched {
			return true
		}
	}

	return false
}

// Pins the .dockerignore semantics the build guard relies on.
func TestDockerignored_MatchesDockerignoreSemantics(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		src     string
		want    bool
	}{
		{
			name:    "exact file name",
			pattern: "README.md",
			src:     "README.md",
			want:    true,
		},
		{
			name:    "extension glob",
			pattern: "*.yaml",
			src:     "config.yaml",
			want:    true,
		},
		{
			name:    "leading slash is relative to context root",
			pattern: "/config.yaml",
			src:     "config.yaml",
			want:    true,
		},
		{
			name:    "directory prefix",
			pattern: "docs",
			src:     "docs/x",
			want:    true,
		},
		{
			name:    "directory prefix with trailing slash",
			pattern: "docs/",
			src:     "docs/nested/x",
			want:    true,
		},
		{
			name:    "different extension does not match",
			pattern: "*.yaml",
			src:     "config.yml",
			want:    false,
		},
		{
			name:    "unrelated file does not match",
			pattern: "docs",
			src:     "README.md",
			want:    false,
		},
		{
			name:    "similar name is not a directory prefix",
			pattern: "docs",
			src:     "docs2/x",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, dockerignored(tt.pattern, tt.src))
		})
	}
}

// Guards the image build: every file the Dockerfile copies from the build
// context must survive .dockerignore, otherwise COPY fails the build.
const r2MinimalYAML = "db:\n  host: localhost\n  user: mandat\n  database: mandat\n"

func TestLoad_R2EnvWiring(t *testing.T) {
	cfg, err := config.Load(writeConfig(t, r2MinimalYAML), envMap(map[string]string{
		"DB_PASSWORD":          "secret",
		"R2_ACCESS_KEY_ID":     "env-key-id",
		"R2_SECRET_ACCESS_KEY": "env-secret-key",
		"R2_ENDPOINT":          "https://env-account.r2.cloudflarestorage.com",
		"R2_BUCKET":            "env-bucket",
		"R2_PUBLIC_BASE_URL":   "https://media.example.com",
	}))

	require.NoError(t, err)
	assert.Equal(t, "env-key-id", cfg.Storage.AccessKeyID)
	assert.Equal(t, "env-secret-key", cfg.Storage.SecretAccessKey)
	assert.Equal(t, "https://env-account.r2.cloudflarestorage.com", cfg.Storage.Endpoint)
	assert.Equal(t, "env-bucket", cfg.Storage.Bucket)
	assert.Equal(t, "https://media.example.com", cfg.Storage.PublicBaseURL)
}

func TestLoad_R2AccountIDDerivesEndpoint(t *testing.T) {
	cfg, err := config.Load(writeConfig(t, r2MinimalYAML), envMap(map[string]string{
		"DB_PASSWORD":          "secret",
		"R2_ACCOUNT_ID":        "abc123",
		"R2_ACCESS_KEY_ID":     "env-key-id",
		"R2_SECRET_ACCESS_KEY": "env-secret-key",
		"R2_BUCKET":            "env-bucket",
		"R2_PUBLIC_BASE_URL":   "https://media.example.com",
	}))

	require.NoError(t, err)
	assert.Equal(t, "https://abc123.r2.cloudflarestorage.com", cfg.Storage.Endpoint,
		"R2_ACCOUNT_ID must derive the endpoint when none is configured")
}

func TestLoad_PartialStorageConfigFails(t *testing.T) {
	cfg, err := config.Load(writeConfig(t, r2MinimalYAML), envMap(map[string]string{
		"DB_PASSWORD": "secret",
		"R2_BUCKET":   "env-bucket",
	}))

	require.Error(t, err, "a partially configured storage section must fail startup")
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "storage")
}

func TestLoad_EmptyStoragePasses(t *testing.T) {
	cfg, err := config.Load(writeConfig(t, r2MinimalYAML), envMap(map[string]string{
		"DB_PASSWORD": "secret",
	}))

	require.NoError(t, err, "an untouched storage section must keep startup working")
	require.NotNil(t, cfg)
	assert.False(t, cfg.Storage.Configured())
	assert.Empty(t, cfg.Storage.Endpoint)
	assert.Empty(t, cfg.Storage.Bucket)
	assert.Empty(t, cfg.Storage.PublicBaseURL)
}

// The shipped config must keep the pasted-image fetcher bounded: a timeout, a
// byte cap, a redirect cap, and the private-address guard switched on.
func TestLoad_DownloadDefaults(t *testing.T) {
	cfg, err := config.Load(shippedConfigPath(t), envMap(map[string]string{"DB_PASSWORD": "secret"}))

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, 15*time.Second, cfg.Download.Timeout)
	assert.Equal(t, int64(10<<20), cfg.Download.MaxBytes)
	assert.Equal(t, 3, cfg.Download.MaxRedirects)
	assert.False(t, cfg.Download.AllowPrivate,
		"the private-address guard must stay on: AllowPrivate is code-only")
}

// A download section in the file may tune the bounds, but it must never lift
// the SSRF guard through a hand-written allowprivate key.
func TestLoad_DownloadOverrides(t *testing.T) {
	const downloadYAML = `
db:
  host: localhost
  user: mandat
  database: mandat
download:
  timeout: 5s
  max_bytes: 1024
  max_redirects: 1
  allowprivate: true
`

	cfg, err := config.Load(writeConfig(t, downloadYAML), envMap(map[string]string{"DB_PASSWORD": "secret"}))

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, 5*time.Second, cfg.Download.Timeout)
	assert.Equal(t, int64(1024), cfg.Download.MaxBytes)
	assert.Equal(t, 1, cfg.Download.MaxRedirects)
	assert.False(t, cfg.Download.AllowPrivate,
		"allowprivate in a config file must not lift the SSRF guard")
}

// Upstream rates URLs are configuration now: omitting the rates section must
// keep the production endpoints as viper defaults.
func TestLoad_RatesDefaults(t *testing.T) {
	const ratesMinimalYAML = "db:\n  host: localhost\n  user: mandat\n  database: mandat\n"

	cfg, err := config.Load(writeConfig(t, ratesMinimalYAML), envMap(map[string]string{"DB_PASSWORD": "secret"}))

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "https://bank.gov.ua/NBU_Exchange/exchange", cfg.Rates.NBUURL,
		"rates.nbu_url must default to the NBU exchange endpoint")
	assert.Equal(t, "https://20.com.ua/api/v1/prices", cfg.Rates.FuelURL,
		"rates.fuel_url must default to the 20.com.ua prices endpoint")
}

// An explicit rates section in the file must win over the defaults.
func TestLoad_RatesOverrides(t *testing.T) {
	const ratesYAML = `
db:
  host: localhost
  user: mandat
  database: mandat
rates:
  nbu_url: https://nbu.test/exchange
  fuel_url: https://fuel.test/api/v1/prices
`

	cfg, err := config.Load(writeConfig(t, ratesYAML), envMap(map[string]string{"DB_PASSWORD": "secret"}))

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "https://nbu.test/exchange", cfg.Rates.NBUURL,
		"rates.nbu_url from the file must override the default")
	assert.Equal(t, "https://fuel.test/api/v1/prices", cfg.Rates.FuelURL,
		"rates.fuel_url from the file must override the default")
}

// Upstream rates URLs are required configuration now: a blank or non-http(s)
// value must fail startup instead of leaving the refresh ticker 503ing forever.
func TestLoad_RatesValidation(t *testing.T) {
	const baseYAML = "db:\n  host: localhost\n  user: mandat\n  database: mandat\n"

	const (
		validNBU  = "rates:\n  nbu_url: https://nbu.test/exchange\n"
		validFuel = "  fuel_url: https://fuel.test/api/v1/prices\n"
	)

	tests := []struct {
		name    string
		rates   string
		wantKey string
		wantMsg string
	}{
		{
			name:    "blank nbu url",
			rates:   "rates:\n  nbu_url: \"\"\n" + validFuel,
			wantKey: "rates.nbu_url",
			wantMsg: "rates.nbu_url is required in the config file",
		},
		{
			name:    "whitespace nbu url",
			rates:   "rates:\n  nbu_url: \"   \"\n" + validFuel,
			wantKey: "rates.nbu_url",
			wantMsg: "rates.nbu_url is required in the config file",
		},
		{
			name:    "scheme-less nbu url",
			rates:   "rates:\n  nbu_url: nbu.test/exchange\n" + validFuel,
			wantKey: "rates.nbu_url",
			wantMsg: `rates.nbu_url must be a valid http(s) URL, got "nbu.test/exchange"`,
		},
		{
			name:    "non-http nbu url",
			rates:   "rates:\n  nbu_url: ftp://x\n" + validFuel,
			wantKey: "rates.nbu_url",
			wantMsg: `rates.nbu_url must be a valid http(s) URL, got "ftp://x"`,
		},
		{
			name:    "blank fuel url",
			rates:   validNBU + "  fuel_url: \"\"\n",
			wantKey: "rates.fuel_url",
			wantMsg: "rates.fuel_url is required in the config file",
		},
		{
			name:    "whitespace fuel url",
			rates:   validNBU + "  fuel_url: \"   \"\n",
			wantKey: "rates.fuel_url",
			wantMsg: "rates.fuel_url is required in the config file",
		},
		{
			name:    "scheme-less fuel url",
			rates:   validNBU + "  fuel_url: fuel.test/api/v1/prices\n",
			wantKey: "rates.fuel_url",
			wantMsg: `rates.fuel_url must be a valid http(s) URL, got "fuel.test/api/v1/prices"`,
		},
		{
			name:    "non-http fuel url",
			rates:   validNBU + "  fuel_url: ftp://x\n",
			wantKey: "rates.fuel_url",
			wantMsg: `rates.fuel_url must be a valid http(s) URL, got "ftp://x"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := config.Load(
				writeConfig(t, baseYAML+tt.rates),
				envMap(map[string]string{"DB_PASSWORD": "secret"}),
			)

			require.Error(t, err, "a broken rates section must fail startup")
			assert.Nil(t, cfg)
			assert.Contains(t, err.Error(), tt.wantKey, "the error must name the offending key")
			assert.Equal(t, tt.wantMsg, err.Error())
		})
	}
}

func TestDockerfileSourcesAreNotDockerignored(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok, "resolve config test file location")

	root := filepath.Join(filepath.Dir(file), "..", "..")

	dockerfile, err := os.ReadFile(filepath.Join(root, "Dockerfile"))
	require.NoError(t, err)

	ignoreContent, err := os.ReadFile(filepath.Join(root, ".dockerignore"))
	require.NoError(t, err)

	sources := dockerfileCopySources(t, string(dockerfile))
	require.Contains(t, sources, "config.yaml", "the Dockerfile must copy the shipped config into the image")

	var ignored []string

	for _, line := range strings.Split(string(ignoreContent), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		ignored = append(ignored, line)
	}

	for _, src := range sources {
		for _, pattern := range ignored {
			if !dockerignored(pattern, src) {
				continue
			}

			assert.Failf(
				t,
				"dockerignored source",
				"Dockerfile copies %q, but .dockerignore pattern %q excludes it from the build context",
				src,
				pattern,
			)
		}
	}
}
