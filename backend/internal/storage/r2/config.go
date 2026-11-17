package r2

import "fmt"

// Config holds Cloudflare R2 connection settings. Endpoint, Bucket and
// PublicBaseURL come from the config file; the key pair always comes from
// the R2_ACCESS_KEY_ID / R2_SECRET_ACCESS_KEY environment variables.
type Config struct {
	Endpoint        string `mapstructure:"endpoint"`
	Bucket          string `mapstructure:"bucket"`
	PublicBaseURL   string `mapstructure:"public_base_url"`
	AccessKeyID     string `mapstructure:"-"`
	SecretAccessKey string `mapstructure:"-"`
}

// Validate fails fast on missing required settings.
func (c *Config) Validate() error {
	switch {
	case c.Endpoint == "":
		return fmt.Errorf("storage.endpoint is required in the config file (or set R2_ACCOUNT_ID)")
	case c.Bucket == "":
		return fmt.Errorf("storage.bucket is required in the config file")
	case c.PublicBaseURL == "":
		return fmt.Errorf("storage.public_base_url is required in the config file (covers are served publicly; the S3 API endpoint is authenticated)")
	case c.AccessKeyID == "":
		return fmt.Errorf("R2_ACCESS_KEY_ID environment variable is required (the key never lives in the config file)")
	case c.SecretAccessKey == "":
		return fmt.Errorf("R2_SECRET_ACCESS_KEY environment variable is required (the secret never lives in the config file)")
	}

	return nil
}

// Configured reports whether any storage setting is present.
// An untouched storage section keeps the legacy no-upload behavior.
func (c *Config) Configured() bool {
	return c.Endpoint != "" || c.Bucket != "" || c.PublicBaseURL != "" ||
		c.AccessKeyID != "" || c.SecretAccessKey != ""
}

// PublicBase is the delivery root of stored objects.
func (c *Config) PublicBase() string {
	if c.PublicBaseURL != "" {
		return c.PublicBaseURL
	}

	return c.Endpoint + "/" + c.Bucket
}
