package service

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rateLimitFakeStore stands in for the Redis counter so tests need no live Redis.
type rateLimitFakeStore struct {
	calls      int
	lastKey    string
	lastWindow time.Duration
	count      int64
	ttl        time.Duration
	err        error
}

// Hit records the call and answers with the canned count, TTL, or error.
func (f *rateLimitFakeStore) Hit(
	_ context.Context,
	key string,
	window time.Duration,
) (int64, time.Duration, error) {
	f.calls++
	f.lastKey = key
	f.lastWindow = window

	if f.err != nil {
		return 0, 0, f.err
	}

	return f.count, f.ttl, nil
}

func newRateLimitTestService(
	store RateLimitStore,
	cfg RateLimitConfig,
) (*RateLimitService, *captureHandler) {
	handler := &captureHandler{}
	logger := slog.New(handler)

	return NewRateLimitService(store, logger, cfg), handler
}

func newScopedRateLimitTestService(
	store RateLimitStore,
	cfg RateLimitConfig,
	scope string,
) (*RateLimitService, *captureHandler) {
	handler := &captureHandler{}
	logger := slog.New(handler)

	return NewScopedRateLimitService(store, logger, cfg, scope), handler
}

func enabledRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{Enabled: true, Requests: 300, Window: time.Minute}
}

func TestRateLimitService_Allow_UnderLimit(t *testing.T) {
	store := &rateLimitFakeStore{count: 42, ttl: 30 * time.Second}
	svc, handler := newRateLimitTestService(store, enabledRateLimitConfig())

	wait, err := svc.Allow(context.Background(), "203.0.113.7")

	require.NoError(t, err)
	assert.Zero(t, wait)
	assert.Equal(t, 1, store.calls)
	assert.Equal(t, "global:203.0.113.7", store.lastKey)
	assert.Equal(t, time.Minute, store.lastWindow)
	assert.Empty(t, handler.records, "an allowed request must not log")
}

func TestRateLimitService_Allow_AtLimit(t *testing.T) {
	store := &rateLimitFakeStore{count: 300, ttl: 12 * time.Second}
	svc, _ := newRateLimitTestService(store, enabledRateLimitConfig())

	wait, err := svc.Allow(context.Background(), "203.0.113.7")

	require.NoError(t, err, "the request that reaches the limit exactly is still allowed")
	assert.Zero(t, wait)
	assert.Equal(t, 1, store.calls)
}

func TestRateLimitService_Allow_OverLimit(t *testing.T) {
	store := &rateLimitFakeStore{count: 301, ttl: 37 * time.Second}
	svc, _ := newRateLimitTestService(store, enabledRateLimitConfig())

	wait, err := svc.Allow(context.Background(), "203.0.113.7")

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrTooManyRequests),
		"expected ErrTooManyRequests, got %v", err)
	assert.Equal(t, 37*time.Second, wait, "the wait time is the store TTL")
	assert.Equal(t, 1, store.calls)
	assert.Equal(t, "global:203.0.113.7", store.lastKey)
}

func TestRateLimitService_Allow_StoreErrorFailsOpen(t *testing.T) {
	store := &rateLimitFakeStore{err: errors.New("redis is down")}
	svc, handler := newRateLimitTestService(store, enabledRateLimitConfig())

	wait, err := svc.Allow(context.Background(), "203.0.113.7")

	require.NoError(t, err, "a Redis failure must let the request through")
	assert.Zero(t, wait)
	assert.True(t, handler.hasFC("rateLimitService_Allow"),
		"the fail-open must still be logged with the function context")
}

func TestRateLimitService_Allow_DisabledNeverCallsStore(t *testing.T) {
	store := &rateLimitFakeStore{count: 1000000, ttl: time.Hour}
	svc, _ := newRateLimitTestService(store, RateLimitConfig{
		Enabled:  false,
		Requests: 300,
		Window:   time.Minute,
	})

	wait, err := svc.Allow(context.Background(), "203.0.113.7")

	require.NoError(t, err)
	assert.Zero(t, wait)
	assert.Zero(t, store.calls, "a disabled limiter must not touch the store")
}

// rateLimitCtxErrStore answers with the context error, like a store
// call interrupted by a client disconnect.
type rateLimitCtxErrStore struct{}

// Hit records nothing and reports the caller's cancellation.
func (s *rateLimitCtxErrStore) Hit(
	ctx context.Context,
	_ string,
	_ time.Duration,
) (int64, time.Duration, error) {
	return 0, 0, ctx.Err()
}

func TestRateLimitService_Allow_CancelledContextSkipsErrorLog(t *testing.T) {
	store := &rateLimitCtxErrStore{}
	svc, handler := newRateLimitTestService(store, enabledRateLimitConfig())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	wait, err := svc.Allow(ctx, "203.0.113.7")

	require.NoError(t, err, "a cancelled request must still fail open")
	assert.Zero(t, wait)
	assert.Zero(t, countLogLevel(handler, slog.LevelError),
		"a client disconnect is not a Redis outage worth an Error log")
}

func TestScopedRateLimitService_Allow_UsesScopePrefix(t *testing.T) {
	store := &rateLimitFakeStore{count: 1, ttl: time.Minute}
	svc, _ := newScopedRateLimitTestService(store, enabledRateLimitConfig(), "login:ip")

	wait, err := svc.Allow(context.Background(), "203.0.113.7")

	require.NoError(t, err)
	assert.Zero(t, wait)
	assert.Equal(t, 1, store.calls)
	assert.Equal(t, "login:ip:203.0.113.7", store.lastKey,
		"a scoped service must namespace its counters under the scope")
}

func TestScopedRateLimitService_Allow_UsesKeyVerbatim(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want string
	}{
		{"case preserved", "Admin", "login:user:Admin"},
		{"ipv6 looking key not masked", "2001:db8::1", "login:user:2001:db8::1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &rateLimitFakeStore{count: 1, ttl: time.Minute}
			svc, _ := newScopedRateLimitTestService(store, enabledRateLimitConfig(), "login:user")

			wait, err := svc.Allow(context.Background(), tt.key)

			require.NoError(t, err)
			assert.Zero(t, wait)
			assert.Equal(t, 1, store.calls)
			assert.Equal(t, tt.want, store.lastKey,
				"Allow must keep the key verbatim, including its case")
		})
	}
}

func TestScopedRateLimitService_Allow_OverLimit(t *testing.T) {
	store := &rateLimitFakeStore{
		count: 301,
		ttl:   25 * time.Second,
	}
	svc, _ := newScopedRateLimitTestService(store, enabledRateLimitConfig(), "login:user")

	wait, err := svc.Allow(context.Background(), "Admin")

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrTooManyRequests),
		"expected ErrTooManyRequests, got %v", err)
	assert.Equal(t, 25*time.Second, wait, "the wait time is the store TTL")
	assert.Equal(t, 1, store.calls)
	assert.Equal(t, "login:user:Admin", store.lastKey)
}

func TestScopedRateLimitService_Allow_DisabledNeverCallsStore(t *testing.T) {
	store := &rateLimitFakeStore{count: 1000000, ttl: time.Hour}
	svc, _ := newScopedRateLimitTestService(store, RateLimitConfig{
		Enabled:  false,
		Requests: 300,
		Window:   time.Minute,
	}, "login:user")

	wait, err := svc.Allow(context.Background(), "Admin")

	require.NoError(t, err)
	assert.Zero(t, wait)
	assert.Zero(t, store.calls, "a disabled limiter must not touch the store")
}

func TestScopedRateLimitService_Allow_StoreErrorFailsOpen(t *testing.T) {
	store := &rateLimitFakeStore{err: errors.New("redis is down")}
	svc, handler := newScopedRateLimitTestService(store, enabledRateLimitConfig(), "login:user")

	wait, err := svc.Allow(context.Background(), "Admin")

	require.NoError(t, err, "a Redis failure must let the request through")
	assert.Zero(t, wait)
	assert.True(t, handler.hasFC("rateLimitService_Allow"),
		"the fail-open must still be logged with the function context")
}
