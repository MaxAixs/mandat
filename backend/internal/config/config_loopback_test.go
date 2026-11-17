package config_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/maxaixs/mandat/backend/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Insecure cookies must stay usable for every loopback origin: the whole
// 127/8 range, bracketed IPv6 loopback and any letter case of localhost.
func TestLoad_SecureCookiesFalse_LoopbackOriginsAccepted(t *testing.T) {
	tests := []struct {
		name    string
		origins []string
	}{
		{"127 of 8 is loopback", []string{"http://127.0.0.2:5173"}},
		{"bracketed ipv6 loopback", []string{"http://[::1]:3000"}},
		{"uppercase scheme and host", []string{"HTTP://LOCALHOST:5173"}},
		{"mixed case host", []string{"http://LocalHost:5173"}},
		{
			name: "all loopback spellings together",
			origins: []string{
				"http://127.0.0.2:5173",
				"http://[::1]:3000",
				"HTTP://LOCALHOST:5173",
				"http://LocalHost:5173",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sb strings.Builder
			sb.WriteString(reviewBaseYAML)
			sb.WriteString("http:\n  secure_cookies: false\n  allowed_origins:\n")
			for _, origin := range tt.origins {
				fmt.Fprintf(&sb, "    - %s\n", origin)
			}

			cfg, err := config.Load(writeConfig(t, sb.String()), reviewEnv(nil))

			require.NoError(t, err, "loopback origins must not require secure cookies")
			require.NotNil(t, cfg)
			assert.Equal(t, tt.origins, cfg.HTTP.AllowedOrigins)
		})
	}
}

// Anything outside loopback still needs secure cookies, even an unroutable
// private address that merely looks local.
func TestLoad_SecureCookiesFalse_NonLoopbackOriginsRejected(t *testing.T) {
	tests := []struct {
		name    string
		origins []string
	}{
		{"production https", []string{"https://mandat.ua"}},
		{"private ipv4", []string{"http://10.0.0.5:5173"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sb strings.Builder
			sb.WriteString(reviewBaseYAML)
			sb.WriteString("http:\n  secure_cookies: false\n  allowed_origins:\n")
			for _, origin := range tt.origins {
				fmt.Fprintf(&sb, "    - %s\n", origin)
			}

			cfg, err := config.Load(writeConfig(t, sb.String()), reviewEnv(nil))

			require.Error(t, err, "a non-loopback origin without secure cookies must fail startup")
			assert.Nil(t, cfg)
			assert.Contains(t, err.Error(), "secure_cookies")
		})
	}
}
