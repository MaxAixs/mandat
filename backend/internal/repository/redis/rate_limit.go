package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/maxaixs/mandat/backend/internal/errs"
	goredis "github.com/redis/go-redis/v9"
)

// rateLimitKeyPrefix namespaces rate limit counters inside the shared Redis database.
const rateLimitKeyPrefix = "ratelimit:"

// rateLimitReplyLen is the number of values the hit script returns: count and PTTL.
const rateLimitReplyLen = 2

// hitScript counts one request and starts the window on the first one.
// INCR and PEXPIRE run atomically, so a counter can never be left without a TTL.
const hitScript = `
local count = redis.call('INCR', KEYS[1])
if count == 1 then
	redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
return {count, redis.call('PTTL', KEYS[1])}
`

// evalCmdable is the slice of the Redis client RateLimitStore needs.
type evalCmdable interface {
	Eval(ctx context.Context, script string, keys []string, args ...any) *goredis.Cmd
}

// RateLimitStore keeps fixed-window request counters in Redis.
type RateLimitStore struct {
	client evalCmdable
}

// NewRateLimitStore creates a RateLimitStore backed by client.
func NewRateLimitStore(client *goredis.Client) *RateLimitStore {
	return &RateLimitStore{client: client}
}

// Hit counts one request for key inside window and returns the count
// together with the time left until the window resets.
func (s *RateLimitStore) Hit(ctx context.Context, key string, window time.Duration) (int64, time.Duration, error) {
	const fc = "rateLimitStore_Hit"

	if window <= 0 {
		return 0, 0, fmt.Errorf("%s: non-positive window: %w", fc, errs.ErrInvalidInput)
	}

	reply, err := s.client.Eval(ctx, hitScript, []string{rateLimitKeyPrefix + key}, window.Milliseconds()).Result()
	if err != nil {
		return 0, 0, fmt.Errorf("%s: %w", fc, err)
	}

	count, ttl, err := parseHitReply(reply)
	if err != nil {
		return 0, 0, fmt.Errorf("%s: %w", fc, err)
	}

	return count, ttl, nil
}

// parseHitReply decodes the {count, pttl_ms} reply of hitScript.
func parseHitReply(reply any) (int64, time.Duration, error) {
	values, ok := reply.([]any)
	if !ok || len(values) != rateLimitReplyLen {
		return 0, 0, errMalformedReply
	}

	count, ok := values[0].(int64)
	if !ok {
		return 0, 0, errMalformedReply
	}

	ttlMs, ok := values[1].(int64)
	if !ok {
		return 0, 0, errMalformedReply
	}

	return count, time.Duration(ttlMs) * time.Millisecond, nil
}
