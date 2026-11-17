package redis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/maxaixs/mandat/backend/internal/errs"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rateLimitFakeCmdable stands in for evalCmdable so tests need no live Redis.
// It records the script call and answers Eval with a canned value or error.
type rateLimitFakeCmdable struct {
	script string
	keys   []string
	args   []any
	calls  int
	val    any
	err    error
}

// Eval records the call and returns the canned reply as a Redis command.
func (f *rateLimitFakeCmdable) Eval(
	_ context.Context,
	script string,
	keys []string,
	args ...any,
) *goredis.Cmd {
	f.calls++
	f.script = script
	f.keys = append([]string(nil), keys...)
	f.args = append([]any(nil), args...)

	cmd := goredis.NewCmd(context.Background())
	if f.err != nil {
		cmd.SetErr(f.err)
	} else {
		cmd.SetVal(f.val)
	}

	return cmd
}

func TestRateLimitStore_Hit_SendsPrefixedKeyAndWindowMs(t *testing.T) {
	fake := &rateLimitFakeCmdable{val: []any{int64(1), int64(60000)}}
	store := &RateLimitStore{client: fake}

	count, ttl, err := store.Hit(context.Background(), "global:203.0.113.7", time.Minute)

	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
	assert.Equal(t, time.Minute, ttl)
	require.Len(t, fake.keys, 1, "the Lua script counts a single key")
	assert.Equal(t, "ratelimit:global:203.0.113.7", fake.keys[0],
		"the store key must carry the ratelimit: prefix")
	require.NotEmpty(t, fake.args, "the window must travel to the script as an argument")
	assert.Equal(t, int64(60000), fake.args[0],
		"the window must be passed to Eval in milliseconds")
	assert.NotEmpty(t, fake.script, "the counting must run as a Lua script")
}

func TestRateLimitStore_Hit_ParsesCountAndTTL(t *testing.T) {
	fake := &rateLimitFakeCmdable{val: []any{int64(7), int64(42500)}}
	store := &RateLimitStore{client: fake}

	count, ttl, err := store.Hit(context.Background(), "global:203.0.113.7", time.Minute)

	require.NoError(t, err)
	assert.Equal(t, int64(7), count, "the first reply element is the request count")
	assert.Equal(t, 42500*time.Millisecond, ttl, "the second reply element is the window TTL")
}

func TestRateLimitStore_Hit_RedisError(t *testing.T) {
	fake := &rateLimitFakeCmdable{err: errors.New("redis is down")}
	store := &RateLimitStore{client: fake}

	count, ttl, err := store.Hit(context.Background(), "global:203.0.113.7", time.Minute)

	require.Error(t, err, "a Redis failure must surface to the caller")
	assert.Zero(t, count)
	assert.Zero(t, ttl)
	assert.False(t, errors.Is(err, errs.ErrInvalidInput),
		"a Redis failure must not look like invalid input, got %v", err)
	assert.Contains(t, err.Error(), "rateLimitStore_Hit",
		"the wrapped error must carry the function context")
}

func TestRateLimitStore_Hit_MalformedReply(t *testing.T) {
	tests := []struct {
		name string
		val  any
	}{
		{"wrong type", "not-a-list"},
		{"too short", []any{int64(1)}},
		{"too long", []any{int64(1), int64(1000), int64(2)}},
		{"wrong count type", []any{"one", int64(1000)}},
		{"wrong ttl type", []any{int64(1), "soon"}},
		{"nil reply", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &rateLimitFakeCmdable{val: tt.val}
			store := &RateLimitStore{client: fake}

			count, ttl, err := store.Hit(context.Background(), "global:203.0.113.7", time.Minute)

			require.Error(t, err, "a malformed Lua reply must fail the hit")
			assert.Zero(t, count)
			assert.Zero(t, ttl)
			assert.False(t, errors.Is(err, errs.ErrInvalidInput),
				"a malformed reply must not look like invalid input, got %v", err)
		})
	}
}

func TestRateLimitStore_Hit_NonPositiveWindow(t *testing.T) {
	tests := []struct {
		name   string
		window time.Duration
	}{
		{"zero", 0},
		{"negative", -time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &rateLimitFakeCmdable{val: []any{int64(1), int64(60000)}}
			store := &RateLimitStore{client: fake}

			count, ttl, err := store.Hit(context.Background(), "global:203.0.113.7", tt.window)

			require.Error(t, err)
			assert.True(t, errors.Is(err, errs.ErrInvalidInput),
				"expected ErrInvalidInput, got %v", err)
			assert.Zero(t, count)
			assert.Zero(t, ttl)
			assert.Zero(t, fake.calls, "an invalid window must never reach Redis")
		})
	}
}
