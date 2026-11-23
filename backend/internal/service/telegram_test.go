package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
	"github.com/maxaixs/mandat/backend/pkg/httpclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// telegramChannelURL is the public channel page the service must fetch.
const telegramChannelURL = "https://t.me/mandat_ua"

// stubTelegramHTTP serves one canned channel page and records every GetText call.
type stubTelegramHTTP struct {
	body      string
	failCalls int
	calls     int
	urls      []string
	maxBytes  []int64
	logger    *slog.Logger
}

var _ TelegramHTTPClient = (*stubTelegramHTTP)(nil)

// GetText fails the first failCalls attempts, then returns the canned page.
func (c *stubTelegramHTTP) GetText(_ context.Context, url string, maxBytes int64) (string, error) {
	c.calls++
	c.urls = append(c.urls, url)
	c.maxBytes = append(c.maxBytes, maxBytes)

	if c.calls <= c.failCalls {
		return "", errConnectionRefused
	}

	return c.body, nil
}

// FetchWithRetries delegates to the real client so attempt budget, timing,
// cancellation and retry logging stay production-faithful.
func (c *stubTelegramHTTP) FetchWithRetries(
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

// stubTelegramStore keeps the latest channel stats in memory.
type stubTelegramStore struct {
	saved     *models.ChannelStats
	saveErr   error
	latestErr error
	saveCalls int
}

var _ TelegramStore = (*stubTelegramStore)(nil)

// Save records the stats unless saveErr forces a failure.
func (s *stubTelegramStore) Save(_ context.Context, stats *models.ChannelStats) error {
	s.saveCalls++

	if s.saveErr != nil {
		return s.saveErr
	}

	s.saved = &models.ChannelStats{Subscribers: stats.Subscribers, UpdatedAt: stats.UpdatedAt}

	return nil
}

// Latest returns the stored stats, or ErrTelegramStatsNotFound when empty.
func (s *stubTelegramStore) Latest(_ context.Context) (*models.ChannelStats, error) {
	if s.latestErr != nil {
		return nil, fmt.Errorf("telegramStore_Latest: %w", s.latestErr)
	}

	if s.saved == nil {
		return nil, fmt.Errorf("telegramStore_Latest: %w", errs.ErrTelegramStatsNotFound)
	}

	return s.saved, nil
}

func newTelegramTestService(
	client *stubTelegramHTTP,
	store *stubTelegramStore,
	fixed time.Time,
) *TelegramService {
	svc := NewTelegramService(client, store, discardLogger(), telegramChannelURL)
	svc.now = func() time.Time { return fixed }
	svc.retryDelays = [3]time.Duration{}

	return svc
}

// TestTelegramRefresh_ParsesSubscriberCount pins the tgme_page_extra parsing:
// the number before subscriber(s), with plain, non-breaking and narrow
// non-breaking spaces between digit groups.
func TestTelegramRefresh_ParsesSubscriberCount(t *testing.T) {
	fixed := time.Date(2026, time.November, 23, 14, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		page string
		want int
	}{
		{
			name: "plain spaces",
			page: `<div class="tgme_page_extra">3 846 subscribers</div>`,
			want: 3846,
		},
		{
			name: "nbsp between groups",
			page: "<div class=\"tgme_page_extra\">3\u00a0846 subscribers</div>",
			want: 3846,
		},
		{
			name: "narrow nbsp between groups",
			page: "<div class=\"tgme_page_extra\">3\u202f846 subscribers</div>",
			want: 3846,
		},
		{
			name: "singular subscriber",
			page: `<div class="tgme_page_extra">1 subscriber</div>`,
			want: 1,
		},
		{
			name: "millions with groups",
			page: `<div class="tgme_page_extra">1 234 567 subscribers</div>`,
			want: 1234567,
		},
		{
			name: "real page markup around the block",
			page: `<html><body><div class="tgme_page_wrap"><div class="tgme_page_extra">3 846 subscribers</div></div></body></html>`,
			want: 3846,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &stubTelegramHTTP{body: tt.page}
			store := &stubTelegramStore{}
			svc := newTelegramTestService(client, store, fixed)

			err := svc.Refresh(context.Background())

			require.NoError(t, err)
			require.Equal(t, 1, client.calls, "one successful attempt must issue exactly one request")
			assert.Equal(t, telegramChannelURL, client.urls[0], "refresh must fetch the channel URL")
			assert.Equal(t, int64(512<<10), client.maxBytes[0], "the channel page fetch must be capped at 512 KiB")
			require.NotNil(t, store.saved, "a parsed count must be stored")
			assert.Equal(t, tt.want, store.saved.Subscribers)
			assert.True(t, store.saved.UpdatedAt.Equal(fixed.UTC()),
				"stored UpdatedAt must be now().UTC(), got %v", store.saved.UpdatedAt)
		})
	}
}

// TestTelegramRefresh_RetryThenSuccess pins that a flaky channel page is
// retried through the shared FetchWithRetries policy.
func TestTelegramRefresh_RetryThenSuccess(t *testing.T) {
	fixed := time.Date(2026, time.November, 23, 14, 0, 0, 0, time.UTC)
	client := &stubTelegramHTTP{
		body:      `<div class="tgme_page_extra">3 846 subscribers</div>`,
		failCalls: 2,
	}
	store := &stubTelegramStore{}
	svc := newTelegramTestService(client, store, fixed)

	err := svc.Refresh(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 3, client.calls, "one attempt plus two retries must succeed")

	for _, url := range client.urls {
		assert.Equal(t, telegramChannelURL, url, "every attempt must fetch the channel URL")
	}

	for _, max := range client.maxBytes {
		assert.Equal(t, int64(512<<10), max, "every attempt must cap the page at 512 KiB")
	}

	require.NotNil(t, store.saved)
	assert.Equal(t, 3846, store.saved.Subscribers)
}

// TestTelegramRefresh_InvalidPageKeepsPreviousValue pins that a page without a
// usable count never overwrites the stored value.
func TestTelegramRefresh_InvalidPageKeepsPreviousValue(t *testing.T) {
	previous := &models.ChannelStats{
		Subscribers: 1000,
		UpdatedAt:   time.Date(2026, time.November, 22, 14, 0, 0, 0, time.UTC),
	}

	tests := []struct {
		name string
		page string
	}{
		{
			name: "page without extra block",
			page: `<html><body><div class="tgme_page">no count here</div></body></html>`,
		},
		{
			name: "zero subscribers",
			page: `<div class="tgme_page_extra">0 subscribers</div>`,
		},
		{
			name: "subscribers without a number",
			page: `<div class="tgme_page_extra">subscribers</div>`,
		},
		{
			name: "wrong word",
			page: `<div class="tgme_page_extra">3 846 members</div>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &stubTelegramHTTP{body: tt.page}
			store := &stubTelegramStore{
				saved: &models.ChannelStats{
					Subscribers: previous.Subscribers,
					UpdatedAt:   previous.UpdatedAt,
				},
			}
			svc := newTelegramTestService(client, store, time.Now())
			svc.retryDelays = [3]time.Duration{}

			err := svc.Refresh(context.Background())

			require.Error(t, err, "an unparsable page must fail the refresh")
			assert.Zero(t, store.saveCalls, "an unparsable page must not write anything")
			require.NotNil(t, store.saved, "the previous value must stay stored")
			assert.Equal(t, previous.Subscribers, store.saved.Subscribers,
				"the stored count must stay untouched")
			assert.True(t, store.saved.UpdatedAt.Equal(previous.UpdatedAt),
				"the stored timestamp must stay untouched")
		})
	}
}

// TestTelegramRefresh_FetchFailureKeepsPreviousValue pins that a failed fetch
// never overwrites the stored value.
func TestTelegramRefresh_FetchFailureKeepsPreviousValue(t *testing.T) {
	previous := &models.ChannelStats{
		Subscribers: 1000,
		UpdatedAt:   time.Date(2026, time.November, 22, 14, 0, 0, 0, time.UTC),
	}
	client := &stubTelegramHTTP{failCalls: 4}
	store := &stubTelegramStore{
		saved: &models.ChannelStats{
			Subscribers: previous.Subscribers,
			UpdatedAt:   previous.UpdatedAt,
		},
	}
	svc := newTelegramTestService(client, store, time.Now())

	err := svc.Refresh(context.Background())

	require.Error(t, err, "a failed fetch must fail the refresh")
	assert.Equal(t, 4, client.calls, "a failed fetch must spend the full attempt budget")
	assert.Zero(t, store.saveCalls, "a failed fetch must not write anything")
	require.NotNil(t, store.saved, "the previous value must stay stored")
	assert.Equal(t, previous.Subscribers, store.saved.Subscribers,
		"the stored count must stay untouched")
}

// TestTelegramRefresh_SaveFailurePropagatesError pins that a storage failure
// fails the refresh and leaves the previous value untouched.
func TestTelegramRefresh_SaveFailurePropagatesError(t *testing.T) {
	previous := &models.ChannelStats{
		Subscribers: 1000,
		UpdatedAt:   time.Date(2026, time.November, 22, 14, 0, 0, 0, time.UTC),
	}

	tests := []struct {
		name     string
		previous *models.ChannelStats
	}{
		{
			name: "previous value kept",
			previous: &models.ChannelStats{
				Subscribers: previous.Subscribers,
				UpdatedAt:   previous.UpdatedAt,
			},
		},
		{
			name:     "nothing stored yet stays empty",
			previous: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			saveErr := errors.New("store is down")
			client := &stubTelegramHTTP{body: `<div class="tgme_page_extra">3 846 subscribers</div>`}
			store := &stubTelegramStore{saveErr: saveErr}
			if tt.previous != nil {
				store.saved = &models.ChannelStats{
					Subscribers: tt.previous.Subscribers,
					UpdatedAt:   tt.previous.UpdatedAt,
				}
			}
			svc := newTelegramTestService(client, store, time.Now())

			err := svc.Refresh(context.Background())

			require.Error(t, err, "a save failure must fail the refresh")
			assert.True(t, errors.Is(err, saveErr), "expected the Save error, got %v", err)
			assert.Equal(t, 1, store.saveCalls, "the parsed count must reach Save once")
			if tt.previous == nil {
				assert.Nil(t, store.saved, "a failed save must not store anything")
				return
			}
			require.NotNil(t, store.saved, "the previous value must stay stored")
			assert.Equal(t, tt.previous.Subscribers, store.saved.Subscribers,
				"the stored count must stay untouched")
			assert.True(t, store.saved.UpdatedAt.Equal(tt.previous.UpdatedAt),
				"the stored timestamp must stay untouched")
		})
	}
}

// TestTelegramStats_ReturnsStoredValue pins that reads are storage-only.
func TestTelegramStats_ReturnsStoredValue(t *testing.T) {
	fixed := time.Date(2026, time.November, 23, 14, 0, 0, 0, time.UTC)
	client := &stubTelegramHTTP{failCalls: 4}
	store := &stubTelegramStore{
		saved: &models.ChannelStats{Subscribers: 3846, UpdatedAt: fixed},
	}
	svc := newTelegramTestService(client, store, fixed)

	stats, err := svc.Stats(context.Background())

	require.NoError(t, err)
	require.NotNil(t, stats)
	assert.Equal(t, 3846, stats.Subscribers)
	assert.True(t, stats.UpdatedAt.Equal(fixed), "Stats must return the stored timestamp")
	assert.Zero(t, client.calls, "a read must not trigger a fetch")
}

// TestTelegramStats_StoreErrorsMapToServiceUnavailable pins that any storage
// failure reads as a 503: nothing stored yet and a broken store alike.
func TestTelegramStats_StoreErrorsMapToServiceUnavailable(t *testing.T) {
	tests := []struct {
		name      string
		latestErr error
	}{
		{
			name:      "nothing stored yet",
			latestErr: errs.ErrTelegramStatsNotFound,
		},
		{
			name:      "storage failure",
			latestErr: errors.New("redis is down"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &stubTelegramHTTP{failCalls: 4}
			store := &stubTelegramStore{latestErr: tt.latestErr}
			svc := newTelegramTestService(client, store, time.Now())

			stats, err := svc.Stats(context.Background())

			require.Error(t, err)
			assert.Empty(t, stats)
			assert.True(t, errors.Is(err, errs.ErrServiceUnavailable),
				"expected ErrServiceUnavailable, got %v", err)
			assert.Zero(t, client.calls, "a failed read must not trigger a fetch")
		})
	}
}

// TestTelegramStartAutoRefresh_ReturnsWhenContextCancelled pins the
// background loop exits with its context.
func TestTelegramStartAutoRefresh_ReturnsWhenContextCancelled(t *testing.T) {
	client := &stubTelegramHTTP{}
	store := &stubTelegramStore{}
	svc := newTelegramTestService(client, store, time.Now())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})

	go func() {
		defer close(done)
		svc.StartAutoRefresh(ctx, time.Hour)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("StartAutoRefresh must return when its context is cancelled")
	}
}
