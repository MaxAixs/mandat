package httptransport

import "time"

// HTTPConfig holds the listen port, timeouts and CORS origins
// of the HTTP layer.
type HTTPConfig struct {
	Port              int           `mapstructure:"port"`
	StartupTimeout    time.Duration `mapstructure:"startup_timeout"`
	ReadHeaderTimeout time.Duration `mapstructure:"read_header_timeout"`
	ReadTimeout       time.Duration `mapstructure:"read_timeout"`
	WriteTimeout      time.Duration `mapstructure:"write_timeout"`
	IdleTimeout       time.Duration `mapstructure:"idle_timeout"`
	ShutdownTimeout   time.Duration `mapstructure:"shutdown_timeout"`
	AllowedOrigins    []string      `mapstructure:"allowed_origins"`
	SecureCookies     bool          `mapstructure:"secure_cookies"`
	SessionTTL        time.Duration `mapstructure:"session_ttl"`

	// TrustedProxies lists the IPs/CIDRs whose client IP headers are honoured;
	// empty means the client IP is always the TCP peer address.
	TrustedProxies []string `mapstructure:"trusted_proxies"`
	// ClientIPHeaders carry the real client IP behind a trusted proxy.
	ClientIPHeaders []string `mapstructure:"client_ip_headers"`
}
