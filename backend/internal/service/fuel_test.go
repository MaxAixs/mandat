package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
	"github.com/maxaixs/mandat/backend/pkg/httpclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fuelPricesURL is the upstream prices endpoint the service must call.
const fuelPricesURL = "https://20.com.ua/api/v1/prices"

// stubFuelHTTP serves one canned prices document and counts every GetJSON call.
type stubFuelHTTP struct {
	body      string
	logger    *slog.Logger
	failCalls int
	calls     int
	urls      []string
}

var _ HTTPClient = (*stubFuelHTTP)(nil)

// GetJSON fails the first failCalls attempts, then decodes the canned body.
func (c *stubFuelHTTP) GetJSON(_ context.Context, url string, out any) error {
	c.calls++
	c.urls = append(c.urls, url)

	if c.calls <= c.failCalls {
		return errConnectionRefused
	}

	if err := json.Unmarshal([]byte(c.body), out); err != nil {
		return fmt.Errorf("stubFuelHTTP_GetJSON: decode body: %w", err)
	}

	return nil
}

// FetchWithRetries delegates to the real client so attempt budget, timing,
// cancellation and retry logging stay production-faithful. The logger is
// parameterised so service-level tests can observe the client's retry records.
func (c *stubFuelHTTP) FetchWithRetries(
	ctx context.Context,
	delays [3]time.Duration,
	errorContext string,
	fn func(context.Context) error,
) error {
	logger := c.logger
	if logger == nil {
		logger = discardLogger()
	}

	return httpclient.New(&http.Client{}, logger).FetchWithRetries(ctx, delays, errorContext, fn)
}

// stubFuelStore is a map-backed FuelStore keyed by calendar day.
type stubFuelStore struct {
	saved     map[string][]*models.Rate
	saveErr   error
	latestErr error
	saveCalls int
}

// Save records the rates for the day unless saveErr forces a failure.
func (s *stubFuelStore) Save(_ context.Context, date time.Time, rates []*models.Rate) error {
	s.saveCalls++

	if s.saveErr != nil {
		return s.saveErr
	}

	if s.saved == nil {
		s.saved = make(map[string][]*models.Rate, 1)
	}

	s.saved[dayKey(date)] = rates

	return nil
}

// Latest returns the newest saved day, or ErrFuelRatesNotFound when empty.
func (s *stubFuelStore) Latest(_ context.Context) ([]*models.Rate, time.Time, error) {
	if s.latestErr != nil {
		return nil, time.Time{}, fmt.Errorf("fuelStore_Latest: %w", s.latestErr)
	}

	var newest time.Time

	for key := range s.saved {
		day, err := time.ParseInLocation("2006-01-02", key, time.UTC)
		if err == nil && day.After(newest) {
			newest = day
		}
	}

	if newest.IsZero() {
		return nil, time.Time{}, fmt.Errorf("fuelStore_Latest: %w", errs.ErrFuelRatesNotFound)
	}

	return s.saved[dayKey(newest)], newest, nil
}

// TestGetFuelRates_ServesRedisWithoutFetching pins that reads are Redis-only.
func TestGetFuelRates_ServesRedisWithoutFetching(t *testing.T) {
	now := time.Now()
	client := &stubFuelHTTP{failCalls: 4}
	store := &stubFuelStore{saved: map[string][]*models.Rate{
		dayKey(now): fullFuel(now),
	}}
	svc := NewFuelService(client, store, discardLogger(), fuelPricesURL)
	svc.now = func() time.Time { return now }
	svc.retryDelays = fastRetryDelays

	rates, err := svc.GetRates(context.Background())

	require.NoError(t, err)
	assertFuelRates(t, rates)
	assert.Equal(t, 0, client.calls, "stored fuel rates must not trigger a fetch")
}

// TestGetFuelRates_RetryThenSuccess pins one attempt plus two retries.
func TestGetFuelRates_RetryThenSuccess(t *testing.T) {
	now := time.Now()
	client := &stubFuelHTTP{body: fuelBody(now), failCalls: 2}
	store := &stubFuelStore{}
	svc := NewFuelService(client, store, discardLogger(), fuelPricesURL)
	svc.now = func() time.Time { return now }
	svc.retryDelays = fastRetryDelays

	err := svc.Refresh(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 3, client.calls, "one attempt plus two retries must succeed")

	for _, url := range client.urls {
		assert.Equal(t, fuelPricesURL, url, "refresh must call the prices endpoint")
	}

	require.Len(t, store.saved[dayKey(now)], 5, "the fetched day must be persisted")

	rates, err := svc.GetRates(context.Background())

	require.NoError(t, err)
	assertFuelRates(t, rates)
}

// TestRefreshFuel_UsesConfiguredAPIURL pins that the injected endpoint is the
// one the service fetches from, so the URL is configurable at runtime.
func TestRefreshFuel_UsesConfiguredAPIURL(t *testing.T) {
	const configuredURL = "https://fuel.test/api/v1/prices"

	now := time.Now()
	client := &stubFuelHTTP{body: fuelBody(now)}
	store := &stubFuelStore{}
	svc := NewFuelService(client, store, discardLogger(), configuredURL)
	svc.now = func() time.Time { return now }
	svc.retryDelays = fastRetryDelays

	err := svc.Refresh(context.Background())

	require.NoError(t, err)
	require.Len(t, client.urls, 1, "one successful attempt must issue exactly one request")
	assert.Equal(t, configuredURL, client.urls[0], "refresh must call the configured endpoint")
	require.Len(t, store.saved[dayKey(now)], 5, "the fetched day must be persisted")
}

// TestRefreshFuel_BlankAPIURLFails pins that an empty injected endpoint fails
// the refresh before any write instead of requesting the zero URL.
func TestRefreshFuel_BlankAPIURLFails(t *testing.T) {
	now := time.Date(2026, 11, 12, 9, 30, 0, 0, time.UTC)
	store := &stubFuelStore{}
	svc := NewFuelService(httpclient.New(&http.Client{}, discardLogger()), store, discardLogger(), "")
	svc.now = func() time.Time { return now }
	svc.retryDelays = fastRetryDelays

	err := svc.Refresh(context.Background())

	require.Error(t, err, "a blank API URL must not refresh successfully")
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable), "expected ErrServiceUnavailable, got %v", err)
	assert.Contains(t, err.Error(), "no fuel rates available", "the fetch must fail, not the store")
	assert.Zero(t, store.saveCalls, "a failed refresh must not write anything")
}

// TestGetFuelRates_NullAverageRejectsDay pins that a null average rejects the day.
func TestGetFuelRates_NullAverageRejectsDay(t *testing.T) {
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	client := &stubFuelHTTP{body: `{"meta":{"updated_at":"` + fuelStamp(now) + `"},` +
		`"averages":{"a95_plus":92.48,"a95":null,"a92":84.68,"diesel":104.31,"lpg":61.15}}`}
	store := &stubFuelStore{saved: map[string][]*models.Rate{
		dayKey(yesterday): fullFuel(yesterday),
	}}
	svc := NewFuelService(client, store, discardLogger(), fuelPricesURL)
	svc.now = func() time.Time { return now }
	svc.retryDelays = fastRetryDelays

	err := svc.Refresh(context.Background())

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable), "expected ErrServiceUnavailable, got %v", err)
	assert.Equal(t, 4, client.calls, "a rejected day must spend the full attempt budget of one attempt plus three retries")
	assert.Zero(t, store.saveCalls, "a day with a missing average must not be saved")
	assert.Equal(t, fullFuel(yesterday), store.saved[dayKey(yesterday)], "the stored snapshot must stay untouched")

	rates, err := svc.GetRates(context.Background())

	require.NoError(t, err, "the previous day must still be served")
	assertFuelRates(t, rates)
}

// TestGetFuelRates_EmptyStoreReturns503 pins the absent-snapshot path.
func TestGetFuelRates_EmptyStoreReturns503(t *testing.T) {
	now := time.Now()
	client := &stubFuelHTTP{failCalls: 4}
	store := &stubFuelStore{}
	svc := NewFuelService(client, store, discardLogger(), fuelPricesURL)
	svc.now = func() time.Time { return now }
	svc.retryDelays = fastRetryDelays

	rates, err := svc.GetRates(context.Background())

	require.Error(t, err)
	assert.Empty(t, rates)
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable), "expected ErrServiceUnavailable, got %v", err)
	assert.Equal(t, 0, client.calls, "an empty store must not trigger a fetch")
}

// TestGetFuelRates_StaleSnapshotReturns503 pins the three-calendar-day window.
func TestGetFuelRates_StaleSnapshotReturns503(t *testing.T) {
	now := time.Now()
	stale := now.AddDate(0, 0, -4)
	client := &stubFuelHTTP{failCalls: 4}
	store := &stubFuelStore{saved: map[string][]*models.Rate{
		dayKey(stale): fullFuel(stale),
	}}
	svc := NewFuelService(client, store, discardLogger(), fuelPricesURL)
	svc.now = func() time.Time { return now }
	svc.retryDelays = fastRetryDelays

	rates, err := svc.GetRates(context.Background())

	require.Error(t, err)
	assert.Empty(t, rates)
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable), "expected ErrServiceUnavailable, got %v", err)
	assert.Equal(t, 0, client.calls, "a stale snapshot must not trigger a fetch")
}

// TestRefreshFuel_TotalFailureLogsFCChain pins that the caller's fc reaches the
// logged error chain, while the client's retry records stay service-fc-free.
func TestRefreshFuel_TotalFailureLogsFCChain(t *testing.T) {
	now := time.Now()
	handler := &captureHandler{}
	client := &stubFuelHTTP{failCalls: 4, logger: slog.New(handler)}
	store := &stubFuelStore{}
	svc := NewFuelService(client, store, slog.New(handler), fuelPricesURL)
	svc.now = func() time.Time { return now }
	svc.retryDelays = fastRetryDelays

	err := svc.Refresh(context.Background())

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable), "expected ErrServiceUnavailable, got %v", err)
	assert.Equal(t, 4, client.calls, "a total failure must spend the full attempt budget of one attempt plus three retries")
	assert.Zero(t, store.saveCalls, "a failed refresh must not write anything")
	matchedErr := assertSomeErrorLogContains(
		t,
		handler,
		"fuelService_fetchWithRetry",
		"fuelService_fetchDay",
	)
	assert.Equal(
		t,
		1,
		strings.Count(matchedErr, "fuelService_fetchWithRetry"),
		"the caller must wrap its fc exactly once, got %q",
		matchedErr,
	)

	assertRetryLogs(
		t,
		handler,
		"fuel fetch retry failed",
		[]int64{2, 3, 4},
	)
}

// assertFuelRates pins the persisted fuel codes, values and display order.
func assertFuelRates(t *testing.T, rates []*models.Rate) {
	t.Helper()

	require.Len(t, rates, 5, "only A95, A92, A98, DIESEL and LPG must be returned")

	wantCodes := []string{"A95", "A92", "A98", "DIESEL", "LPG"}
	wantValues := []float64{88.74, 84.68, 92.48, 104.31, 61.15}

	for i, code := range wantCodes {
		assert.Equal(t, code, rates[i].Code, "rates must be ordered A95, A92, A98, DIESEL, LPG")
		assert.InDelta(t, wantValues[i], rates[i].Rate, 1e-9)
	}
}

// fullFuel is the persisted snapshot: five prices in display order.
func fullFuel(date time.Time) []*models.Rate {
	stamp := date.Format("02.01.2006")

	return []*models.Rate{
		{Code: "A95", Rate: 88.74, Date: stamp},
		{Code: "A92", Rate: 84.68, Date: stamp},
		{Code: "A98", Rate: 92.48, Date: stamp},
		{Code: "DIESEL", Rate: 104.31, Date: stamp},
		{Code: "LPG", Rate: 61.15, Date: stamp},
	}
}

// fuelStamp renders date as the live upstream does: RFC3339 at a fixed
// EET wall clock, e.g. 2026-11-11T13:39:00+02:00.
func fuelStamp(date time.Time) string {
	stamp := time.Date(
		date.Year(), date.Month(), date.Day(),
		13, 39, 0, 0,
		time.FixedZone("EET", 2*60*60),
	)

	return stamp.Format("2006-01-02T15:04:05-07:00")
}

// fuelBody is a complete upstream prices document stamped with date.
func fuelBody(date time.Time) string {
	return fmt.Sprintf(
		`{"meta":{"updated_at":%q},`+
			`"averages":{"a95_plus":92.48,"a95":88.74,"a92":84.68,"diesel":104.31,"lpg":61.15}}`,
		fuelStamp(date),
	)
}
