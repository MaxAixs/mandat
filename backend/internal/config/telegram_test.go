package config_test

import (
	"testing"
	"time"

	"github.com/maxaixs/mandat/backend/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Omitted telegram settings must fall back to the public channel page and an
// hourly refresh.
func TestLoad_TelegramDefaults(t *testing.T) {
	cfg, err := config.Load(writeConfig(t, reviewBaseYAML), reviewEnv(nil))

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "https://t.me/mandat_ua", cfg.Telegram.ChannelURL,
		"telegram.channel_url must default to the public channel page")
	assert.Equal(t, time.Hour, cfg.Telegram.RefreshInterval,
		"telegram.refresh_interval must default to an hourly refresh")
}

// Only https://t.me/<name> pages carry a subscriber count, so anything else
// must fail startup.
func TestLoad_TelegramChannelURLValidation(t *testing.T) {
	tests := []struct {
		name       string
		channelURL string
		wantErr    string
	}{
		{
			name:       "public channel accepted",
			channelURL: "https://t.me/mandat_ua",
			wantErr:    "",
		},
		{
			name:       "plain http rejected",
			channelURL: "http://t.me/x",
			wantErr:    "telegram.channel_url",
		},
		{
			name:       "other host rejected",
			channelURL: "https://evil.com/mandat_ua",
			wantErr:    "telegram.channel_url",
		},
		{
			name:       "missing channel name rejected",
			channelURL: "https://t.me/",
			wantErr:    "telegram.channel_url",
		},
		{
			name:       "nested path rejected",
			channelURL: "https://t.me/a/b",
			wantErr:    "telegram.channel_url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := reviewBaseYAML + "telegram:\n  channel_url: " + tt.channelURL + "\n"

			cfg, err := config.Load(writeConfig(t, content), reviewEnv(nil))

			if tt.wantErr == "" {
				require.NoError(t, err)
				require.NotNil(t, cfg)
				assert.Equal(t, tt.channelURL, cfg.Telegram.ChannelURL)

				return
			}

			require.Error(t, err, "a non-t.me channel URL must fail startup")
			assert.Nil(t, cfg)
			assert.Contains(t, err.Error(), tt.wantErr,
				"the error must name the offending key")
		})
	}
}

// An explicitly emptied channel_url or a zero refresh_interval must fail
// startup instead of silently polling a broken cadence.
func TestLoad_TelegramExplicitEmptyValuesFailStartup(t *testing.T) {
	tests := []struct {
		name    string
		section string
		wantErr string
	}{
		{
			name:    "empty channel url rejected",
			section: "telegram:\n  channel_url: \"\"\n",
			wantErr: "telegram.channel_url",
		},
		{
			name:    "zero refresh interval rejected",
			section: "telegram:\n  refresh_interval: 0s\n",
			wantErr: "telegram.refresh_interval",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := config.Load(writeConfig(t, reviewBaseYAML+tt.section), reviewEnv(nil))

			require.Error(t, err, "an explicitly emptied telegram value must fail startup")
			assert.Nil(t, cfg)
			assert.Contains(t, err.Error(), tt.wantErr,
				"the error must name the offending key")
		})
	}
}

// The hourly ticker must not hammer the channel page faster than once a minute.
func TestLoad_TelegramRefreshIntervalValidation(t *testing.T) {
	tests := []struct {
		name     string
		interval string
		wantErr  string
	}{
		{
			name:     "thirty seconds rejected",
			interval: "30s",
			wantErr:  "telegram.refresh_interval",
		},
		{
			name:     "below one minute rejected",
			interval: "59s",
			wantErr:  "telegram.refresh_interval",
		},
		{
			name:     "one minute accepted",
			interval: "1m",
			wantErr:  "",
		},
		{
			name:     "one hour accepted",
			interval: "1h",
			wantErr:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := reviewBaseYAML + "telegram:\n  refresh_interval: " + tt.interval + "\n"

			cfg, err := config.Load(writeConfig(t, content), reviewEnv(nil))

			if tt.wantErr == "" {
				require.NoError(t, err)
				require.NotNil(t, cfg)

				return
			}

			require.Error(t, err, "a sub-minute telegram refresh must fail startup")
			assert.Nil(t, cfg)
			assert.Contains(t, err.Error(), tt.wantErr,
				"the error must name the offending key")
		})
	}
}
