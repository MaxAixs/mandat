package redis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// telegramFakeCmdable stands in for redisCmdable so tests need no live Redis.
// It records the expiration passed to Set, which is how tests assert the TTL.
type telegramFakeCmdable struct {
	entries map[string]fakeEntry
	setErr  error
}

// Set stores value under key and remembers the requested expiration.
func (f *telegramFakeCmdable) Set(_ context.Context, key string, value any, exp time.Duration) *goredis.StatusCmd {
	if f.setErr != nil {
		return goredis.NewStatusResult("", f.setErr)
	}

	var payload string

	switch v := value.(type) {
	case string:
		payload = v
	case []byte:
		payload = string(v)
	default:
		return goredis.NewStatusResult("", errors.New("fake: unsupported value type"))
	}

	f.entries[key] = fakeEntry{val: payload, exp: exp}

	return goredis.NewStatusResult("", nil)
}

// Get returns the stored value, or goredis.Nil when the key is absent.
func (f *telegramFakeCmdable) Get(_ context.Context, key string) *goredis.StringCmd {
	entry, ok := f.entries[key]
	if !ok {
		return goredis.NewStringResult("", goredis.Nil)
	}

	return goredis.NewStringResult(entry.val, nil)
}

func newTelegramFakeCmdable() *telegramFakeCmdable {
	return &telegramFakeCmdable{entries: map[string]fakeEntry{}}
}

func TestTelegramStore_SaveThenLatest(t *testing.T) {
	updatedAt := time.Date(2026, time.November, 23, 14, 0, 0, 0, time.UTC)
	stats := &models.ChannelStats{Subscribers: 3846, UpdatedAt: updatedAt}

	fake := newTelegramFakeCmdable()
	store := NewTelegramStore(fake)

	require.NoError(t, store.Save(context.Background(), stats))

	entry, ok := fake.entries["telegram:channel"]
	require.True(t, ok, "Save must write telegram:channel")
	assert.Equal(t, time.Duration(0), entry.exp, "the channel stats must not expire")
	assert.Contains(t, entry.val, `"subscribers":3846`)

	got, err := store.Latest(context.Background())

	require.NoError(t, err)
	require.NotNil(t, got, "Latest must return the stored stats")
	assert.Equal(t, 3846, got.Subscribers, "Latest must return the stored count")
	assert.True(t, got.UpdatedAt.Equal(updatedAt),
		"Latest must round-trip UpdatedAt, got %v", got.UpdatedAt)
}

func TestTelegramStore_LatestEmptyReturnsNotFound(t *testing.T) {
	store := NewTelegramStore(newTelegramFakeCmdable())

	stats, err := store.Latest(context.Background())

	require.Error(t, err, "Latest must fail when nothing was ever stored")
	assert.Empty(t, stats)
	assert.True(t, errors.Is(err, errs.ErrTelegramStatsNotFound),
		"expected ErrTelegramStatsNotFound, got %v", err)
}

func TestTelegramStore_LatestCorruptJSONReturnsNotFound(t *testing.T) {
	fake := newTelegramFakeCmdable()
	fake.entries["telegram:channel"] = fakeEntry{val: "not-json{", exp: 0}

	store := NewTelegramStore(fake)

	stats, err := store.Latest(context.Background())

	require.Error(t, err, "Latest must fail on a corrupt payload")
	assert.Empty(t, stats)
	assert.True(t, errors.Is(err, errs.ErrTelegramStatsNotFound),
		"expected ErrTelegramStatsNotFound, got %v", err)
}
