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
}
