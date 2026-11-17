package config_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/maxaixs/mandat/backend/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// reviewBaseYAML is the smallest valid file: every omitted section must fall
// back to production-safe defaults.
const reviewBaseYAML = "db:\n  host: localhost\n  user: mandat\n  database: mandat\n"

func reviewEnv(extra map[string]string) func(string) string {
	vars := map[string]string{"DB_PASSWORD": "secret"}
	for k, v := range extra {
		vars[k] = v
	}

	return envMap(vars)
}

// Omitted sections must keep production-safe defaults: a 30s write deadline
// above the 15s image fetch, daily rate refreshes, a 10s provider timeout and
// a 5-requests-per-5-minutes login limit.
func TestLoad_ReviewFixesDefaults(t *testing.T) {
	cfg, err := config.Load(writeConfig(t, reviewBaseYAML), reviewEnv(nil))

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, 30*time.Second, cfg.HTTP.WriteTimeout)
	assert.Equal(t, 24*time.Hour, cfg.Rates.RefreshInterval)
	assert.Equal(t, 10*time.Second, cfg.Rates.HTTPTimeout)
	assert.True(t, cfg.LoginRateLimit.Enabled, "the login limit must be on by default")
	assert.Equal(t, int64(5), cfg.LoginRateLimit.Requests)
	assert.Equal(t, 5*time.Minute, cfg.LoginRateLimit.Window)
}

// The image fetch must fit inside the server write deadline, otherwise the
// admin upload always times out at the HTTP layer.
func TestLoad_DownloadTimeoutBelowWriteTimeout(t *testing.T) {
	tests := []struct {
		name     string
		write    string
		download string
		wantErr  string
	}{
		{
			name:     "equal timeouts rejected",
			write:    "  write_timeout: 10s\n",
			download: "  timeout: 10s\n",
			wantErr:  "download.timeout",
		},
		{
			name:     "download above write rejected",
			write:    "  write_timeout: 10s\n",
			download: "  timeout: 15s\n",
			wantErr:  "download.timeout",
		},
		{
			name:     "download below write allowed",
			write:    "  write_timeout: 10s\n",
			download: "  timeout: 5s\n",
			wantErr:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := reviewBaseYAML + "http:\n" + tt.write + "download:\n" + tt.download

			cfg, err := config.Load(writeConfig(t, content), reviewEnv(nil))

			if tt.wantErr == "" {
				require.NoError(t, err)
				require.NotNil(t, cfg)

				return
			}

			require.Error(t, err, "a download that outlasts the write deadline must fail startup")
			assert.Nil(t, cfg)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// HTTP_SECURE_COOKIES=true must flip a file that ships false for local dev.
func TestLoad_SecureCookiesEnvOverride(t *testing.T) {
	content := reviewBaseYAML + "http:\n  secure_cookies: false\n"

	cfg, err := config.Load(writeConfig(t, content), reviewEnv(map[string]string{"HTTP_SECURE_COOKIES": "true"}))

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.True(t, cfg.HTTP.SecureCookies, "HTTP_SECURE_COOKIES=true must override the file value")
}

// Anything but a bool must fail startup instead of silently staying false.
func TestLoad_SecureCookiesEnvInvalid(t *testing.T) {
	cfg, err := config.Load(writeConfig(t, reviewBaseYAML), reviewEnv(map[string]string{"HTTP_SECURE_COOKIES": "nope"}))

	require.Error(t, err, "an invalid HTTP_SECURE_COOKIES value must fail startup")
	assert.Nil(t, cfg)
}

// A comma-separated env list replaces the file origins; entries are trimmed.
func TestLoad_AllowedOriginsEnvOverride(t *testing.T) {
	cfg, err := config.Load(writeConfig(t, reviewBaseYAML), reviewEnv(map[string]string{
		"HTTP_SECURE_COOKIES":  "true",
		"HTTP_ALLOWED_ORIGINS": " https://mandat.ua , https://admin.mandat.ua ",
	}))

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, []string{"https://mandat.ua", "https://admin.mandat.ua"}, cfg.HTTP.AllowedOrigins)
}

// Insecure cookies may only serve loopback origins, so a production deploy
// without HTTPS fails fast instead of leaking the session cookie.
func TestLoad_SecureCookiesOriginGuard(t *testing.T) {
	tests := []struct {
		name    string
		secure  bool
		origins []string
		wantErr string
	}{
		{
			name:    "https origin with insecure cookies rejected",
			secure:  false,
			origins: []string{"https://mandat.ua"},
			wantErr: "secure_cookies",
		},
		{
			name:    "loopback origins with insecure cookies allowed",
			secure:  false,
			origins: []string{"http://localhost:5173", "http://127.0.0.1:3000", "http://[::1]:5173"},
			wantErr: "",
		},
		{
			name:    "https origin with secure cookies allowed",
			secure:  true,
			origins: []string{"https://mandat.ua"},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sb strings.Builder
			sb.WriteString(reviewBaseYAML)
			fmt.Fprintf(&sb, "http:\n  secure_cookies: %v\n  allowed_origins:\n", tt.secure)
			for _, origin := range tt.origins {
				fmt.Fprintf(&sb, "    - %s\n", origin)
			}

			cfg, err := config.Load(writeConfig(t, sb.String()), reviewEnv(nil))

			if tt.wantErr == "" {
				require.NoError(t, err)
				require.NotNil(t, cfg)
				assert.Equal(t, tt.origins, cfg.HTTP.AllowedOrigins)

				return
			}

			require.Error(t, err, "a non-local origin without secure cookies must fail startup")
			assert.Nil(t, cfg)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// Every allowed origin must be an http(s) URL with a host.
func TestLoad_AllowedOriginURLValidation(t *testing.T) {
	for _, origin := range []string{"not a url", "ftp://x"} {
		t.Run(origin, func(t *testing.T) {
			content := reviewBaseYAML + "http:\n  secure_cookies: true\n  allowed_origins:\n    - " + origin + "\n"

			cfg, err := config.Load(writeConfig(t, content), reviewEnv(nil))

			require.Error(t, err, "an invalid allowed origin must fail startup")
			assert.Nil(t, cfg)
		})
	}
}

// The refresh ticker must not spin faster than once a minute, and every
// provider request needs a positive timeout.
func TestLoad_RatesIntervalsValidation(t *testing.T) {
	tests := []struct {
		name    string
		section string
		wantErr string
	}{
		{
			name:    "refresh interval below minimum",
			section: "  refresh_interval: 30s\n",
			wantErr: "refresh_interval",
		},
		{
			name:    "http timeout zero",
			section: "  http_timeout: 0s\n",
			wantErr: "http_timeout",
		},
		{
			name:    "http timeout negative",
			section: "  http_timeout: -5s\n",
			wantErr: "http_timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := reviewBaseYAML + "rates:\n" + tt.section

			cfg, err := config.Load(writeConfig(t, content), reviewEnv(nil))

			require.Error(t, err, "a broken rates interval must fail startup")
			assert.Nil(t, cfg)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// The login limit is tuned like the global one, but disabling it must skip
// the tuning checks entirely.
func TestLoad_LoginRateLimitValidation(t *testing.T) {
	tests := []struct {
		name    string
		section string
		wantErr string
	}{
		{
			name:    "zero requests rejected",
			section: "  enabled: true\n  requests: 0\n",
			wantErr: "login_rate_limit",
		},
		{
			name:    "tiny window rejected",
			section: "  enabled: true\n  window: 500ms\n",
			wantErr: "login_rate_limit",
		},
		{
			name:    "disabled with zero requests allowed",
			section: "  enabled: false\n  requests: 0\n",
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := reviewBaseYAML + "login_rate_limit:\n" + tt.section

			cfg, err := config.Load(writeConfig(t, content), reviewEnv(nil))

			if tt.wantErr == "" {
				require.NoError(t, err, "tuning values must be ignored while disabled")
				require.NotNil(t, cfg)
				assert.False(t, cfg.LoginRateLimit.Enabled)

				return
			}

			require.Error(t, err, "a broken login limit must fail startup")
			assert.Nil(t, cfg)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// The shipped file must stay loadable: with the new download/write-timeout
// rule this pins write_timeout above download.timeout, and with the origin
// guard it pins loopback-only origins next to secure_cookies: false.
func TestShippedConfig_LoadsWithPassword(t *testing.T) {
	cfg, err := config.Load(shippedConfigPath(t), reviewEnv(nil))

	require.NoError(t, err, "the shipped config.yaml must load with DB_PASSWORD set")
	require.NotNil(t, cfg)
}
