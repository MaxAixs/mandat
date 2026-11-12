package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	neturl "net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
	"github.com/maxaixs/mandat/backend/pkg/httpclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fastRetryDelays keeps the retry path instant in tests.
var fastRetryDelays = [3]time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}

// nbuRatesURL is the default NBU exchange endpoint without query parameters;
// production now injects it from config (rates.nbu_url).
const nbuRatesURL = "https://bank.gov.ua/NBU_Exchange/exchange"

// errConnectionRefused is what the fake NBU endpoint returns when unreachable.
var errConnectionRefused = errors.New("connection refused")

// stubJSONClient serves raw NBU JSON per date and counts every GetJSON call.
type stubJSONClient struct {
	bodies    map[string]string
	logger    *slog.Logger
	urls      []string
	failCalls int
	calls     int
}

var _ HTTPClient = (*stubJSONClient)(nil)

// GetJSON fails the first failCalls attempts, then decodes the body for the
// query date; a date without a canned body behaves like an unreachable day.
func (c *stubJSONClient) GetJSON(_ context.Context, url string, out any) error {
	c.calls++
	c.urls = append(c.urls, url)

	if c.calls <= c.failCalls {
		return errConnectionRefused
	}

	body, ok := c.bodies[nbuQueryDate(url)]
	if !ok {
		return errConnectionRefused
	}

	if err := json.Unmarshal([]byte(body), out); err != nil {
		return fmt.Errorf("stubJSONClient_GetJSON: decode body: %w", err)
	}

	return nil
}

// FetchWithRetries delegates to the real client so attempt budget, timing,
// cancellation and retry logging stay production-faithful. The logger is
// parameterised so service-level tests can observe the client's retry records.
func (c *stubJSONClient) FetchWithRetries(
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

// nbuQueryDate extracts the NBU date query parameter from url.
func nbuQueryDate(rawURL string) string {
	parsed, err := neturl.Parse(rawURL)
	if err != nil {
		return ""
	}

	return parsed.Query().Get("date")
}

// stubExchangeStore is a map-backed ExchangeStore keyed by calendar day.
type stubExchangeStore struct {
	saved     map[string][]*models.Rate
	saveErr   error
	latestErr error
	saveCalls int
}

// Save records the rates for the day unless saveErr forces a failure.
func (s *stubExchangeStore) Save(_ context.Context, date time.Time, rates []*models.Rate) error {
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

// Latest returns the newest saved day, or ErrExchangeRatesNotFound when empty.
func (s *stubExchangeStore) Latest(_ context.Context) ([]*models.Rate, time.Time, error) {
	if s.latestErr != nil {
		return nil, time.Time{}, fmt.Errorf("exchangeStore_Latest: %w", s.latestErr)
	}

	var newest time.Time

	for key := range s.saved {
		day, err := time.ParseInLocation("2006-01-02", key, time.UTC)
		if err == nil && day.After(newest) {
			newest = day
		}
	}

	if newest.IsZero() {
		return nil, time.Time{}, fmt.Errorf("exchangeStore_Latest: %w", errs.ErrExchangeRatesNotFound)
	}

	return s.saved[dayKey(newest)], newest, nil
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestGetRates_ServesRedisWithoutFetching(t *testing.T) {
	now := time.Now()
	client := &stubJSONClient{}
	store := &stubExchangeStore{saved: map[string][]*models.Rate{
		dayKey(now): wantedRates(now),
	}}
	svc := NewExchangeService(client, store, discardLogger(), nbuRatesURL)
	svc.now = func() time.Time { return now }
	svc.retryDelays = fastRetryDelays

	rates, err := svc.GetRates(context.Background())

	require.NoError(t, err)
	assertWantedRates(t, rates, dayKey(now))
	assert.Equal(t, 0, client.calls, "fresh stored rates must not trigger a fetch")
}

func TestGetRates_RetryThenSuccess(t *testing.T) {
	now := time.Now()
	client := &stubJSONClient{
		bodies:    map[string]string{nbuDate(now): fullBody(now)},
		failCalls: 2,
	}
	store := &stubExchangeStore{}
	svc := NewExchangeService(client, store, discardLogger(), nbuRatesURL)
	svc.now = func() time.Time { return now }
	svc.retryDelays = fastRetryDelays

	err := svc.Refresh(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 3, client.calls, "one attempt plus two retries must succeed")
	require.Len(t, store.saved[dayKey(now)], 3, "the fetched day must be persisted")

	rates, err := svc.GetRates(context.Background())

	require.NoError(t, err)
	assertWantedRates(t, rates, nbuDate(now))
}

// TestRefresh_UsesConfiguredBaseURL pins that the injected base URL is the one
// the service fetches from, so the endpoint is configurable at runtime.
func TestRefresh_UsesConfiguredBaseURL(t *testing.T) {
	const configuredBase = "https://nbu.test/exchange"

	now := time.Now()
	client := &stubJSONClient{bodies: map[string]string{nbuDate(now): fullBody(now)}}
	store := &stubExchangeStore{}
	svc := NewExchangeService(client, store, discardLogger(), configuredBase)
	svc.now = func() time.Time { return now }
	svc.retryDelays = fastRetryDelays

	err := svc.Refresh(context.Background())

	require.NoError(t, err)
	require.Len(t, client.urls, 1, "one successful attempt must issue exactly one request")
	assert.True(
		t,
		strings.HasPrefix(client.urls[0], configuredBase),
		"expected a request to %q, got %q",
		configuredBase,
		client.urls[0],
	)
	assert.Equal(t, nbuDate(now), nbuQueryDate(client.urls[0]), "the request must still carry the date query")
	require.Len(t, store.saved[dayKey(now)], 3, "the fetched day must be persisted")
}

// TestRefresh_BaseURLTrailingSlash pins the URL join: a trailing slash on the
// configured base URL must not produce a double slash or drop the producer's
// query format.
func TestRefresh_BaseURLTrailingSlash(t *testing.T) {
	now := time.Date(2026, 11, 12, 9, 30, 0, 0, time.UTC)
	client := &stubJSONClient{bodies: map[string]string{nbuDate(now): fullBody(now)}}
	store := &stubExchangeStore{}
	svc := NewExchangeService(client, store, discardLogger(), "https://nbu.test/exchange/")
	svc.now = func() time.Time { return now }
	svc.retryDelays = fastRetryDelays

	err := svc.Refresh(context.Background())

	require.NoError(t, err)
	require.Len(t, client.urls, 1, "one successful attempt must issue exactly one request")
	assert.Equal(t, "https://nbu.test/exchange?json&date=12.11.2026", client.urls[0],
		"the trailing slash must collapse into a single slash before the query")
	require.Len(t, store.saved[dayKey(now)], 3, "the fetched day must be persisted")
}

// TestRefresh_BlankBaseURLFails pins that an empty injected endpoint fails the
// refresh before any write instead of requesting a relative URL.
func TestRefresh_BlankBaseURLFails(t *testing.T) {
	now := time.Date(2026, 11, 12, 9, 30, 0, 0, time.UTC)
	store := &stubExchangeStore{}
	svc := NewExchangeService(httpclient.New(&http.Client{}, discardLogger()), store, discardLogger(), "")
	svc.now = func() time.Time { return now }
	svc.retryDelays = fastRetryDelays

	err := svc.Refresh(context.Background())

	require.Error(t, err, "a blank base URL must not refresh successfully")
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable), "expected ErrServiceUnavailable, got %v", err)
	assert.Contains(t, err.Error(), "no exchange rates available", "the fetch must fail, not the store")
	assert.Zero(t, store.saveCalls, "a failed refresh must not write anything")
}

// TestGetRates_NoWalkBackToNBU pins that a failed today refresh never asks NBU
// for a past date: Redis history is the only fallback.
func TestGetRates_NoWalkBackToNBU(t *testing.T) {
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	client := &stubJSONClient{failCalls: 4}
	store := &stubExchangeStore{saved: map[string][]*models.Rate{
		dayKey(yesterday): wantedRates(yesterday),
	}}
	svc := NewExchangeService(client, store, discardLogger(), nbuRatesURL)
	svc.now = func() time.Time { return now }
	svc.retryDelays = fastRetryDelays

	err := svc.Refresh(context.Background())

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable), "expected ErrServiceUnavailable, got %v", err)
	assert.Equal(t, 4, client.calls, "today gets one attempt plus three retries and no past-date fetch")
	assert.Zero(t, store.saveCalls, "a failed refresh must not write anything")
	assert.Equal(t, wantedRates(yesterday), store.saved[dayKey(yesterday)], "the Redis snapshot must stay untouched")

	rates, err := svc.GetRates(context.Background())

	require.NoError(t, err, "Redis history must still serve the last stored day")
	assertWantedRates(t, rates, dayKey(yesterday))
}

// TestGetRates_IncompleteTodayFallsBackToRedis pins that an incomplete today is
// not retried and never triggered a past-date fetch: Redis history is served.
func TestGetRates_IncompleteTodayFallsBackToRedis(t *testing.T) {
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	client := &stubJSONClient{bodies: map[string]string{
		nbuDate(now): nbuBody(now,
			[2]any{"EUR", 50.1},
			[2]any{"USD", 45.1},
			[2]any{"JPY", 0.3},
		),
	}}
	store := &stubExchangeStore{saved: map[string][]*models.Rate{
		dayKey(yesterday): wantedRates(yesterday),
	}}
	svc := NewExchangeService(client, store, discardLogger(), nbuRatesURL)
	svc.now = func() time.Time { return now }
	svc.retryDelays = fastRetryDelays

	err := svc.Refresh(context.Background())

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable), "expected ErrServiceUnavailable, got %v", err)
	assert.Equal(t, 1, client.calls, "incomplete data is not retried and must not walk back")
	assert.Zero(t, store.saveCalls, "an incomplete day must not overwrite the Redis snapshot")

	rates, err := svc.GetRates(context.Background())

	require.NoError(t, err, "Redis history must still serve the last stored day")
	assertWantedRates(t, rates, dayKey(yesterday))
}

func TestGetRates_StaleRedisReturns503(t *testing.T) {
	now := time.Now()
	stale := now.AddDate(0, 0, -4)
	client := &stubJSONClient{}
	store := &stubExchangeStore{saved: map[string][]*models.Rate{
		dayKey(stale): wantedRates(stale),
	}}
	svc := NewExchangeService(client, store, discardLogger(), nbuRatesURL)
	svc.now = func() time.Time { return now }
	svc.retryDelays = fastRetryDelays

	rates, err := svc.GetRates(context.Background())

	require.Error(t, err)
	assert.Empty(t, rates)
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable), "expected ErrServiceUnavailable, got %v", err)
}

func TestGetRates_EmptyStoreReturns503(t *testing.T) {
	now := time.Now()
	client := &stubJSONClient{}
	store := &stubExchangeStore{}
	svc := NewExchangeService(client, store, discardLogger(), nbuRatesURL)
	svc.now = func() time.Time { return now }
	svc.retryDelays = fastRetryDelays

	rates, err := svc.GetRates(context.Background())

	require.Error(t, err)
	assert.Empty(t, rates)
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable), "expected ErrServiceUnavailable, got %v", err)
}

func TestGetRates_SaveFailureFailsRefresh(t *testing.T) {
	now := time.Now()
	client := &stubJSONClient{bodies: map[string]string{nbuDate(now): fullBody(now)}}
	store := &stubExchangeStore{saveErr: errors.New("store is down")}
	svc := NewExchangeService(client, store, discardLogger(), nbuRatesURL)
	svc.now = func() time.Time { return now }
	svc.retryDelays = fastRetryDelays

	err := svc.Refresh(context.Background())

	require.Error(t, err)
	assert.Equal(t, 1, store.saveCalls, "the fetched day must reach Save")

	rates, err := svc.GetRates(context.Background())

	require.Error(t, err)
	assert.Empty(t, rates)
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable), "expected ErrServiceUnavailable, got %v", err)
}

func TestGetRates_ParallelReads(t *testing.T) {
	now := time.Now()
	client := &stubJSONClient{}
	store := &stubExchangeStore{saved: map[string][]*models.Rate{
		dayKey(now): wantedRates(now),
	}}
	svc := NewExchangeService(client, store, discardLogger(), nbuRatesURL)
	svc.now = func() time.Time { return now }
	svc.retryDelays = fastRetryDelays

	const readers = 16

	results := make([][]*models.Rate, readers)
	errsByReader := make([]error, readers)

	var wg sync.WaitGroup

	wg.Add(readers)

	for i := 0; i < readers; i++ {
		go func(i int) {
			defer wg.Done()

			results[i], errsByReader[i] = svc.GetRates(context.Background())
		}(i)
	}

	wg.Wait()

	for i := 0; i < readers; i++ {
		require.NoError(t, errsByReader[i])
		assert.Equal(t, results[0], results[i], "every reader must see the same rates")
	}

	assertWantedRates(t, results[0], dayKey(now))
	assert.Equal(t, 0, client.calls, "parallel reads of stored rates must not fetch")
}

func TestGetRates_FutureDatedRedisServed(t *testing.T) {
	now := time.Now()
	tomorrow := now.AddDate(0, 0, 1)
	client := &stubJSONClient{}
	store := &stubExchangeStore{saved: map[string][]*models.Rate{
		dayKey(tomorrow): wantedRates(tomorrow),
	}}
	svc := NewExchangeService(client, store, discardLogger(), nbuRatesURL)
	svc.now = func() time.Time { return now }
	svc.retryDelays = fastRetryDelays

	rates, err := svc.GetRates(context.Background())

	require.NoError(t, err)
	assertWantedRates(t, rates, dayKey(tomorrow))
	assert.Equal(t, 0, client.calls, "future-dated stored rates must not trigger a fetch")
}

func assertWantedRates(t *testing.T, rates []*models.Rate, wantDate string) {
	t.Helper()

	require.Len(t, rates, 3, "only EUR, USD and PLN must be returned")

	wantCodes := []string{"EUR", "USD", "PLN"}
	wantValues := []float64{50.1, 45.1, 11.1}

	for i, code := range wantCodes {
		assert.Equal(t, code, rates[i].Code, "rates must be ordered EUR, USD, PLN")
		assert.InDelta(t, wantValues[i], rates[i].Rate, 1e-9)
		assert.Equal(t, wantDate, rates[i].Date)
	}
}

func dayKey(date time.Time) string {
	return date.Format("2006-01-02")
}

// nbuDate renders date in the NBU wire format used by the date query.
func nbuDate(date time.Time) string {
	return date.Format("02.01.2006")
}

// nbuBody renders a raw NBU JSON document from currency code / amount pairs.
func nbuBody(date time.Time, rates ...[2]any) string {
	stamp := nbuDate(date)
	items := make([]string, 0, len(rates))

	for _, rate := range rates {
		items = append(items, fmt.Sprintf(
			`{"CurrencyCodeL":%q,"Units":1,"Amount":%v,"StartDate":%q}`,
			rate[0],
			rate[1],
			stamp,
		))
	}

	return "[" + strings.Join(items, ",") + "]"
}

// fullBody is an NBU document: it carries more codes than are wanted.
func fullBody(date time.Time) string {
	return nbuBody(date,
		[2]any{"EUR", 50.1},
		[2]any{"USD", 45.1},
		[2]any{"PLN", 11.1},
		[2]any{"JPY", 0.3},
	)
}

// fullRates is a provider response: it carries more codes than are wanted.
func fullRates(date time.Time) []*models.Rate {
	day := dayKey(date)

	return []*models.Rate{
		{Code: "EUR", Rate: 50.1, Date: day},
		{Code: "USD", Rate: 45.1, Date: day},
		{Code: "PLN", Rate: 11.1, Date: day},
		{Code: "JPY", Rate: 0.3, Date: day},
	}
}

// wantedRates is what production persists: the wanted codes in display order.
func wantedRates(date time.Time) []*models.Rate {
	day := dayKey(date)

	return []*models.Rate{
		{Code: "EUR", Rate: 50.1, Date: day},
		{Code: "USD", Rate: 45.1, Date: day},
		{Code: "PLN", Rate: 11.1, Date: day},
	}
}

// countLogLevel counts records logged at exactly level.
func countLogLevel(h *captureHandler, level slog.Level) int {
	count := 0

	for _, record := range h.records {
		if record.Level == level {
			count++
		}
	}

	return count
}

// TestGetRates_WindowIgnoresServerTimezone pins the window to UTC calendar
// days: a day-3 snapshot is fresh even when the server clock is west of UTC.
func TestGetRates_WindowIgnoresServerTimezone(t *testing.T) {
	now := time.Date(2026, 11, 11, 20, 0, 0, 0, time.FixedZone("EST", -5*3600))
	day3 := now.AddDate(0, 0, -3)
	client := &stubJSONClient{}
	store := &stubExchangeStore{saved: map[string][]*models.Rate{
		dayKey(day3): fullRates(day3),
	}}
	svc := NewExchangeService(client, store, discardLogger(), nbuRatesURL)
	svc.now = func() time.Time { return now }
	svc.retryDelays = fastRetryDelays

	rates, err := svc.GetRates(context.Background())

	require.NoError(t, err, "a three-day-old snapshot must be served regardless of server timezone")
	require.Len(t, rates, 4, "the stored snapshot must be returned as persisted")
	assert.Equal(t, dayKey(day3), rates[0].Date)
	assert.Equal(t, 0, client.calls, "serving stored rates must not fetch")
}

// TestGetRates_AbsentLogsWarnNotError pins the expected fallback level.
func TestGetRates_AbsentLogsWarnNotError(t *testing.T) {
	now := time.Now()
	handler := &captureHandler{}
	store := &stubExchangeStore{}
	svc := NewExchangeService(&stubJSONClient{}, store, slog.New(handler), nbuRatesURL)
	svc.now = func() time.Time { return now }
	svc.retryDelays = fastRetryDelays

	rates, err := svc.GetRates(context.Background())

	require.Error(t, err)
	assert.Empty(t, rates)
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable), "expected ErrServiceUnavailable, got %v", err)
	assert.Zero(t, countLogLevel(handler, slog.LevelError), "an absent snapshot is expected and must not be logged at Error")
	assert.GreaterOrEqual(t, countLogLevel(handler, slog.LevelWarn), 1, "an absent snapshot must be logged at Warn")
}

// TestGetRates_StoreFailureLogsError pins that a real store failure stays Error.
func TestGetRates_StoreFailureLogsError(t *testing.T) {
	now := time.Now()
	handler := &captureHandler{}
	store := &stubExchangeStore{latestErr: errors.New("redis is down")}
	svc := NewExchangeService(&stubJSONClient{}, store, slog.New(handler), nbuRatesURL)
	svc.now = func() time.Time { return now }
	svc.retryDelays = fastRetryDelays

	rates, err := svc.GetRates(context.Background())

	require.Error(t, err)
	assert.Empty(t, rates)
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable), "expected ErrServiceUnavailable, got %v", err)
	assert.GreaterOrEqual(t, countLogLevel(handler, slog.LevelError), 1, "a real store failure must be logged at Error")
}

// TestGetRates_LatestStoreErrorReturns503 pins the Redis-down read path.
func TestGetRates_LatestStoreErrorReturns503(t *testing.T) {
	now := time.Now()
	store := &stubExchangeStore{latestErr: errors.New("redis is down")}
	svc := NewExchangeService(&stubJSONClient{}, store, discardLogger(), nbuRatesURL)
	svc.now = func() time.Time { return now }
	svc.retryDelays = fastRetryDelays

	rates, err := svc.GetRates(context.Background())

	require.Error(t, err)
	assert.Empty(t, rates)
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable), "expected ErrServiceUnavailable, got %v", err)
}

// TestRefresh_TotalFailureLogsFCChain pins that the caller's fc reaches the
// logged error chain, while the client's retry records stay service-fc-free.
func TestRefresh_TotalFailureLogsFCChain(t *testing.T) {
	now := time.Now()
	handler := &captureHandler{}
	client := &stubJSONClient{failCalls: 4, logger: slog.New(handler)}
	store := &stubExchangeStore{}
	svc := NewExchangeService(client, store, slog.New(handler), nbuRatesURL)
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
		"exchangeService_fetchWithRetry",
		"exchangeService_fetchDay",
	)
	assert.Equal(
		t,
		1,
		strings.Count(matchedErr, "exchangeService_fetchWithRetry"),
		"the caller must wrap its fc exactly once, got %q",
		matchedErr,
	)

	assertRetryLogs(
		t,
		handler,
		"exchange fetch retry failed",
		[]int64{2, 3, 4},
	)
}

// ctxBlockingJSONClient fails as soon as the context is done.
type ctxBlockingJSONClient struct{}

// GetJSON waits for cancellation and reports it, never serving a body.
func (c *ctxBlockingJSONClient) GetJSON(ctx context.Context, _ string, _ any) error {
	<-ctx.Done()

	return ctx.Err()
}

// FetchWithRetries delegates to the real client so the cancellation test
// exercises the production retry loop.
func (c *ctxBlockingJSONClient) FetchWithRetries(
	ctx context.Context,
	delays [3]time.Duration,
	errorContext string,
	fn func(context.Context) error,
) error {
	return httpclient.New(&http.Client{}, discardLogger()).FetchWithRetries(ctx, delays, errorContext, fn)
}

// TestService_RefreshCtxCanceled pins that a canceled context aborts before sleeping.
func TestService_RefreshCtxCanceled(t *testing.T) {
	svc := NewExchangeService(&ctxBlockingJSONClient{}, &stubExchangeStore{}, discardLogger(), nbuRatesURL)
	svc.now = time.Now
	svc.retryDelays = fastRetryDelays

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	err := svc.Refresh(ctx)
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Less(t, elapsed, 5*time.Second, "a canceled context must abort Refresh without sleeping")
}
