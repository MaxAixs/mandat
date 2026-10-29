// Package redis provides the Redis client construction and its configuration.
// The password is injected from the environment, never from the config file.
package redis

import (
	"context"
	"fmt"

	goredis "github.com/redis/go-redis/v9"
)

// NewClient connects to Redis and verifies the connection with a ping.
func NewClient(ctx context.Context, cfg Config) (*goredis.Client, error) {
	client := goredis.NewClient(&goredis.Options{
		Addr:        cfg.Addr,
		Password:    cfg.Password,
		DB:          cfg.DB,
		DialTimeout: cfg.DialTimeout,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()

		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return client, nil
}
