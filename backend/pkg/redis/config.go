package redis

import "time"

// Config holds Redis connection settings.
type Config struct {
	Addr        string        `mapstructure:"addr"`
	Password    string        `mapstructure:"-"`
	DB          int           `mapstructure:"db"`
	DialTimeout time.Duration `mapstructure:"dial_timeout"`
}
