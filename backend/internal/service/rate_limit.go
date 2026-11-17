package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/maxaixs/mandat/backend/internal/errs"
)

// globalRateLimitScope scopes the counters of the global per-client limit.
const globalRateLimitScope = "global"

// RateLimitStore is the counter storage used by RateLimitService.
type RateLimitStore interface {
	Hit(ctx context.Context, key string, window time.Duration) (int64, time.Duration, error)
}

// RateLimitConfig is the global per-client request limit.
type RateLimitConfig struct {
	Enabled  bool          `mapstructure:"enabled"`
	Requests int64         `mapstructure:"requests"`
	Window   time.Duration `mapstructure:"window"`
}

// RateLimitService applies the global per-client request limit.
type RateLimitService struct {
	store  RateLimitStore
	logger *slog.Logger
	cfg    RateLimitConfig
	scope  string
}

// NewRateLimitService creates a RateLimitService over store.
func NewRateLimitService(store RateLimitStore, logger *slog.Logger, cfg RateLimitConfig) *RateLimitService {
	return NewScopedRateLimitService(store, logger, cfg, globalRateLimitScope)
}

// NewScopedRateLimitService creates a RateLimitService whose counters live under scope.
func NewScopedRateLimitService(
	store RateLimitStore,
	logger *slog.Logger,
	cfg RateLimitConfig,
	scope string,
) *RateLimitService {
	return &RateLimitService{store: store, logger: logger, cfg: cfg, scope: scope}
}

// Allow counts one request for key in the service scope and returns
// errs.ErrTooManyRequests with the time to wait once key is over the limit.
// Store failures let the request through.
func (s *RateLimitService) Allow(ctx context.Context, key string) (time.Duration, error) {
	const fc = "rateLimitService_Allow"

	if !s.cfg.Enabled {
		return 0, nil
	}

	count, ttl, err := s.store.Hit(ctx, s.scope+":"+key, s.cfg.Window)
	if err != nil {
		if ctx.Err() != nil {
			return 0, nil
		}

		// Fail open: a Redis outage must not take the whole site down.
		s.logger.ErrorContext(ctx, "rate limit check failed, request allowed", "fc", fc, "error", err)

		return 0, nil
	}

	if count > s.cfg.Requests {
		return ttl, fmt.Errorf("%s: %w", fc, errs.ErrTooManyRequests)
	}

	return 0, nil
}
