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

// fuelFakeCmdable stands in for redisCmdable so tests need no live Redis.
// It records the expiration passed to Set, which is how tests assert the TTL.
type fuelFakeCmdable struct {
	entries map[string]fakeEntry
	setErr  error
}

// Set stores value under key and remembers the requested expiration.
func (f *fuelFakeCmdable) Set(_ context.Context, key string, value any, exp time.Duration) *goredis.StatusCmd {
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
func (f *fuelFakeCmdable) Get(_ context.Context, key string) *goredis.StringCmd {
	entry, ok := f.entries[key]
	if !ok {
		return goredis.NewStringResult("", goredis.Nil)
	}

	return goredis.NewStringResult(entry.val, nil)
}

func newFuelFakeCmdable() *fuelFakeCmdable {
	return &fuelFakeCmdable{entries: map[string]fakeEntry{}}
}

func TestFuelStore_SaveThenLatest(t *testing.T) {
	day := time.Date(2026, time.November, 11, 0, 0, 0, 0, time.UTC)
	rates := []*models.Rate{
		{Code: "A95", Rate: 59.9, Date: "11.11.2026"},
		{Code: "A92", Rate: 55.4, Date: "11.11.2026"},
		{Code: "A98", Rate: 63.2, Date: "11.11.2026"},
		{Code: "DIESEL", Rate: 58.7, Date: "11.11.2026"},
		{Code: "LPG", Rate: 27.5, Date: "11.11.2026"},
	}

	fake := newFuelFakeCmdable()
	store := NewFuelStore(fake)

	require.NoError(t, store.Save(context.Background(), day, rates))

	dayEntry, ok := fake.entries["fuel:rates:2026-11-11"]
	require.True(t, ok, "Save must write fuel:rates:2026-11-11")
	assert.Equal(t, 72*time.Hour, dayEntry.exp, "dated rates must expire after 72h")
	assert.Contains(t, dayEntry.val, `"date":"11.11.2026"`)

	latestEntry, ok := fake.entries["fuel:latest"]
	require.True(t, ok, "Save must write fuel:latest")
	assert.Equal(t, 72*time.Hour, latestEntry.exp, "latest pointer must expire after 72h")
	assert.Equal(t, "2026-11-11", latestEntry.val, "latest pointer holds the day as 2006-01-02")

	got, gotDay, err := store.Latest(context.Background())

	require.NoError(t, err)
	require.Len(t, got, 5, "Latest must return the stored rates")

	wantCodes := []string{"A95", "A92", "A98", "DIESEL", "LPG"}
	wantRates := []float64{59.9, 55.4, 63.2, 58.7, 27.5}

	for i, code := range wantCodes {
		assert.Equal(t, code, got[i].Code, "rates must keep the stored order")
		assert.InDelta(t, wantRates[i], got[i].Rate, 1e-9)
	}

	assert.Equal(t, 2026, gotDay.Year())
	assert.Equal(t, time.November, gotDay.Month())
	assert.Equal(t, 11, gotDay.Day())
}

func TestFuelStore_LatestAbsent(t *testing.T) {
	store := NewFuelStore(newFuelFakeCmdable())

	rates, _, err := store.Latest(context.Background())

	require.Error(t, err, "Latest must fail when nothing was ever stored")
	assert.Empty(t, rates)
	assert.True(t, errors.Is(err, errs.ErrFuelRatesNotFound), "expected ErrFuelRatesNotFound, got %v", err)
}

func TestFuelStore_LatestPointerWithoutEntry(t *testing.T) {
	fake := newFuelFakeCmdable()
	fake.entries["fuel:latest"] = fakeEntry{val: "2026-11-11", exp: 72 * time.Hour}

	store := NewFuelStore(fake)

	rates, _, err := store.Latest(context.Background())

	require.Error(t, err, "a pointer without its dated entry is not enough data")
	assert.Empty(t, rates)
	assert.True(t, errors.Is(err, errs.ErrFuelRatesNotFound), "expected ErrFuelRatesNotFound, got %v", err)
}

func TestFuelStore_SaveError(t *testing.T) {
	fake := &fuelFakeCmdable{
		entries: map[string]fakeEntry{},
		setErr:  errors.New("redis is down"),
	}
	store := NewFuelStore(fake)

	err := store.Save(
		context.Background(),
		time.Date(2026, time.November, 11, 0, 0, 0, 0, time.UTC),
		[]*models.Rate{{Code: "A95", Rate: 59.9, Date: "11.11.2026"}},
	)

	require.Error(t, err)
	assert.False(t, errors.Is(err, errs.ErrFuelRatesNotFound), "a Redis failure must not look like an empty cache")
}
