package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
	"github.com/maxaixs/mandat/backend/pkg/httpclient"
)

// telegramPageMaxBytes caps the channel page download; the page is about 10 KB.
const telegramPageMaxBytes = 512 << 10

// telegramSubscribers finds «3 846 subscribers» in the page's tgme_page_extra block; digit
// groups may be split by regular, non-breaking or narrow non-breaking spaces.
var telegramSubscribers = regexp.MustCompile(
	`class="tgme_page_extra"[^>]*>\s*([0-9][0-9 \x{00a0}\x{202f}]*?)[\s\x{00a0}\x{202f}]+subscribers?\s*<`,
)

// TelegramHTTPClient fetches the public channel page with the shared retry policy.
type TelegramHTTPClient interface {
	GetText(ctx context.Context, url string, maxBytes int64) (string, error)
	FetchWithRetries(ctx context.Context, delays [3]time.Duration, errorContext string, fn func(context.Context) error) error
}

// TelegramStore persists the latest channel stats.
// The service owns this contract; internal/repository/redis implements it.
type TelegramStore interface {
	Save(ctx context.Context, stats *models.ChannelStats) error
	// Latest returns the stored stats or errs.ErrTelegramStatsNotFound.
	Latest(ctx context.Context) (*models.ChannelStats, error)
}

// TelegramService refreshes the channel subscriber count into Redis and serves it from there.
// Requests never fetch Telegram: they only read the stored value.
type TelegramService struct {
	http        TelegramHTTPClient
	store       TelegramStore
	logger      *slog.Logger
	now         func() time.Time
	retryDelays [3]time.Duration
	channelURL  string

	refreshMu sync.Mutex
}

// NewTelegramService creates a TelegramService reading the public page at channelURL.
func NewTelegramService(
	httpClient TelegramHTTPClient,
	store TelegramStore,
	logger *slog.Logger,
	channelURL string,
) *TelegramService {
	return &TelegramService{
		http:        httpClient,
		store:       store,
		logger:      logger,
		now:         time.Now,
		retryDelays: httpclient.DefaultDelays,
		channelURL:  channelURL,
	}
}

// StartAutoRefresh refreshes the count every interval until ctx is done.
func (s *TelegramService) StartAutoRefresh(ctx context.Context, interval time.Duration) {
	const fc = "telegramService_StartAutoRefresh"

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.Refresh(ctx); err != nil {
				s.logger.ErrorContext(ctx, "failed to refresh telegram subscribers", "fc", fc, "error", err)
			}
		}
	}
}

// Refresh fetches the channel page with retries, parses the subscriber count and
// stores it. A failed fetch or an unparsable page keeps the previous value.
func (s *TelegramService) Refresh(ctx context.Context) error {
	const fc = "telegramService_Refresh"

	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	var subscribers int
	fetch := func(ctx context.Context) error {
		page, err := s.http.GetText(ctx, s.channelURL, telegramPageMaxBytes)
		if err != nil {
			return err
		}

		subscribers, err = parseSubscribers(page)

		return err
	}
	err := s.http.FetchWithRetries(ctx, s.retryDelays, "telegram subscribers fetch retry failed", fetch)
	if err != nil {
		return fmt.Errorf("%s: %w", fc, err)
	}

	stats := &models.ChannelStats{Subscribers: subscribers, UpdatedAt: s.now().UTC()}
	if err := s.store.Save(ctx, stats); err != nil {
		return fmt.Errorf("%s: %w", fc, err)
	}

	return nil
}

// Stats returns the stored channel stats; nothing stored yet is errs.ErrServiceUnavailable.
func (s *TelegramService) Stats(ctx context.Context) (*models.ChannelStats, error) {
	const fc = "telegramService_Stats"

	stats, err := s.store.Latest(ctx)
	if err != nil {
		if errors.Is(err, errs.ErrTelegramStatsNotFound) {
			s.logger.WarnContext(ctx, "no telegram subscribers stored", "fc", fc)
		} else {
			s.logger.ErrorContext(ctx, "failed to read telegram subscribers", "fc", fc, "error", err)
		}

		return nil, fmt.Errorf("%s: %w", fc, errs.ErrServiceUnavailable)
	}

	return stats, nil
}

// parseSubscribers reads the positive subscriber count from the channel page.
func parseSubscribers(page string) (int, error) {
	match := telegramSubscribers.FindStringSubmatch(page)
	if match == nil {
		return 0, errors.New("subscriber count not found on the channel page")
	}

	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}

		return -1
	}, match[1])

	count, err := strconv.Atoi(digits)
	if err != nil || count <= 0 {
		return 0, fmt.Errorf("invalid subscriber count %q", match[1])
	}

	return count, nil
}
