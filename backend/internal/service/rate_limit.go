package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"time"

	"github.com/maxaixs/mandat/backend/internal/errs"
)

// globalRateLimitKeyPrefix scopes the counters of the global per-client limit.
const globalRateLimitKeyPrefix = "global:"

// ipv6SubscriberBits is the prefix one IPv6 subscriber usually owns,
// so rotating addresses inside it does not reset the limit.
const ipv6SubscriberBits = 64

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
}

// NewRateLimitService creates a RateLimitService over store.
func NewRateLimitService(store RateLimitStore, logger *slog.Logger, cfg RateLimitConfig) *RateLimitService {
	return &RateLimitService{store: store, logger: logger, cfg: cfg}
}

// Allow returns errs.ErrTooManyRequests together with the time to wait
// when clientIP is over the limit. Store failures let the request through.
func (s *RateLimitService) Allow(ctx context.Context, clientIP string) (time.Duration, error) {
	const fc = "rateLimitService_Allow"

	if !s.cfg.Enabled {
		return 0, nil
	}

	count, ttl, err := s.store.Hit(ctx, globalRateLimitKeyPrefix+clientKey(clientIP), s.cfg.Window)
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

// clientKey identifies a client: IPv4 as is, IPv6 by its /64 prefix,
// anything unparsable verbatim.
func clientKey(clientIP string) string {
	addr, err := netip.ParseAddr(clientIP)
	if err != nil {
		return clientIP
	}

	addr = addr.Unmap().WithZone("")
	if addr.Is4() {
		return addr.String()
	}

	return netip.PrefixFrom(addr, ipv6SubscriberBits).Masked().String()
}
