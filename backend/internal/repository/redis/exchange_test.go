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

// exchangeFakeCmdable stands in for redisCmdable so tests need no live Redis.
// It records the expiration passed to Set, which is how tests assert the TTL.
type exchangeFakeCmdable struct {
	entries       map[string]fakeEntry
	setErr        error
	getErr        error
	setCalls      int
	failSecondSet bool
}

// fakeEntry is one stored value together with the TTL requested for it.
type fakeEntry struct {
	val string
	exp time.Duration
}

// Set stores value under key and remembers the requested expiration.
func (f *exchangeFakeCmdable) Set(_ context.Context, key string, value any, exp time.Duration) *goredis.StatusCmd {
	f.setCalls++

	if f.failSecondSet && f.setCalls == 2 {
		return goredis.NewStatusResult("", errors.New("fake: second set failed"))
	}

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
func (f *exchangeFakeCmdable) Get(_ context.Context, key string) *goredis.StringCmd {
	if f.getErr != nil {
		return goredis.NewStringResult("", f.getErr)
	}

	entry, ok := f.entries[key]
	if !ok {
		return goredis.NewStringResult("", goredis.Nil)
	}

	return goredis.NewStringResult(entry.val, nil)
}

func newExchangeFakeCmdable() *exchangeFakeCmdable {
	return &exchangeFakeCmdable{entries: map[string]fakeEntry{}}
}

func TestExchangeStore_SaveThenLatest(t *testing.T) {
	day := time.Date(2026, time.November, 11, 0, 0, 0, 0, time.UTC)
	rates := []*models.ExchangeRate{
		{Code: "EUR", Rate: 50.1, Date: "11.11.2026"},
		{Code: "USD", Rate: 45.1, Date: "11.11.2026"},
		{Code: "PLN", Rate: 11.1, Date: "11.11.2026"},
	}

	fake := newExchangeFakeCmdable()
	store := NewExchangeStore(fake)

	require.NoError(t, store.Save(context.Background(), day, rates))

	dayEntry, ok := fake.entries["exchange:rates:2026-11-11"]
	require.True(t, ok, "Save must write exchange:rates:2026-11-11")
	assert.Equal(t, 72*time.Hour, dayEntry.exp, "dated rates must expire after 72h")
	assert.Contains(t, dayEntry.val, `"date":"11.11.2026"`)

	latestEntry, ok := fake.entries["exchange:rates:latest"]
	require.True(t, ok, "Save must write exchange:rates:latest")
	assert.Equal(t, 72*time.Hour, latestEntry.exp, "latest pointer must expire after 72h")
	assert.Equal(t, "2026-11-11", latestEntry.val, "latest pointer holds the day as 2006-01-02")

	got, gotDay, err := store.Latest(context.Background())

	require.NoError(t, err)
	require.Len(t, got, 3, "Latest must return the stored rates")

	wantCodes := []string{"EUR", "USD", "PLN"}
	wantRates := []float64{50.1, 45.1, 11.1}

	for i, code := range wantCodes {
		assert.Equal(t, code, got[i].Code, "rates must keep the stored order")
		assert.InDelta(t, wantRates[i], got[i].Rate, 1e-9)
	}

	assert.Equal(t, 2026, gotDay.Year())
	assert.Equal(t, time.November, gotDay.Month())
	assert.Equal(t, 11, gotDay.Day())
}

func TestExchangeStore_LatestAbsent(t *testing.T) {
	store := NewExchangeStore(newExchangeFakeCmdable())

	rates, _, err := store.Latest(context.Background())

	require.Error(t, err, "Latest must fail when nothing was ever stored")
	assert.Empty(t, rates)
	assert.True(t, errors.Is(err, errs.ErrExchangeRatesNotFound), "expected ErrExchangeRatesNotFound, got %v", err)
}

func TestExchangeStore_LatestPointerWithoutEntry(t *testing.T) {
	fake := newExchangeFakeCmdable()
	fake.entries["exchange:rates:latest"] = fakeEntry{val: "2026-11-11", exp: 72 * time.Hour}

	store := NewExchangeStore(fake)

	rates, _, err := store.Latest(context.Background())

	require.Error(t, err, "a pointer without its dated entry is not enough data")
	assert.Empty(t, rates)
	assert.True(t, errors.Is(err, errs.ErrExchangeRatesNotFound), "expected ErrExchangeRatesNotFound, got %v", err)
}

func TestExchangeStore_SaveError(t *testing.T) {
	fake := &exchangeFakeCmdable{
		entries: map[string]fakeEntry{},
		setErr:  errors.New("redis is down"),
	}
	store := NewExchangeStore(fake)

	err := store.Save(
		context.Background(),
		time.Date(2026, time.November, 11, 0, 0, 0, 0, time.UTC),
		[]*models.ExchangeRate{{Code: "EUR", Rate: 50.1, Date: "11.11.2026"}},
	)

	require.Error(t, err)
	assert.False(t, errors.Is(err, errs.ErrExchangeRatesNotFound), "a Redis failure must not look like an empty cache")
}

// TestExchangeStore_LatestGetError pins that a read failure is not "not found".
func TestExchangeStore_LatestGetError(t *testing.T) {
	fake := newExchangeFakeCmdable()
	fake.getErr = errors.New("redis read failed")

	store := NewExchangeStore(fake)

	rates, _, err := store.Latest(context.Background())

	require.Error(t, err)
	assert.Empty(t, rates)
	assert.False(t, errors.Is(err, errs.ErrExchangeRatesNotFound), "a Redis read failure must not look like an empty cache")
}

// TestExchangeStore_SaveSecondSetFails pins that a failed latest pointer fails Save.
func TestExchangeStore_SaveSecondSetFails(t *testing.T) {
	fake := newExchangeFakeCmdable()
	fake.failSecondSet = true

	store := NewExchangeStore(fake)

	err := store.Save(
		context.Background(),
		time.Date(2026, time.November, 11, 0, 0, 0, 0, time.UTC),
		[]*models.ExchangeRate{{Code: "EUR", Rate: 50.1, Date: "11.11.2026"}},
	)

	require.Error(t, err, "a failed latest pointer must fail Save")
	assert.Equal(t, 2, fake.setCalls, "the dated key is written before the latest pointer")
}
