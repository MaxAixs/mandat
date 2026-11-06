package r2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validConfig returns a Config with every required field populated.
func validConfig() Config {
	return Config{
		Endpoint:        "https://account.r2.cloudflarestorage.com",
		Bucket:          "mandat-media",
		PublicBaseURL:   "https://media.example.com",
		AccessKeyID:     "access-key",
		SecretAccessKey: "secret-key",
	}
}

func TestConfig_Validate_RejectsEmptyBucket(t *testing.T) {
	cfg := validConfig()
	cfg.Bucket = ""

	err := cfg.Validate()

	require.Error(t, err, "an empty bucket must be rejected")
}

func TestConfig_Validate_RejectsEmptyEndpointOrSecrets(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{
			name:   "empty endpoint",
			mutate: func(c *Config) { c.Endpoint = "" },
		},
		{
			name:   "empty access key id",
			mutate: func(c *Config) { c.AccessKeyID = "" },
		},
		{
			name:   "empty secret access key",
			mutate: func(c *Config) { c.SecretAccessKey = "" },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.mutate(&cfg)

			err := cfg.Validate()

			require.Error(t, err, "a missing required field must be rejected")
		})
	}
}

func TestConfig_Validate_AcceptsFullConfig(t *testing.T) {
	cfg := validConfig()

	err := cfg.Validate()

	require.NoError(t, err, "a fully populated config must be accepted")
}

func TestConfig_Validate_RejectsEmptyPublicBaseURL(t *testing.T) {
	cfg := validConfig()
	cfg.PublicBaseURL = ""

	err := cfg.Validate()

	require.Error(t, err, "covers are served publicly, so PublicBaseURL is required")
}

func TestConfig_Configured(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want bool
	}{
		{
			name: "empty config",
			cfg:  Config{},
			want: false,
		},
		{
			name: "endpoint only",
			cfg:  Config{Endpoint: "https://account.r2.cloudflarestorage.com"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.cfg.Configured())
		})
	}
}
