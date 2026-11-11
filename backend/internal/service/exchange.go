package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
)

const (
	// Exchange refresh policy: today gets one attempt plus three retries with
	// growing pauses. Past days are never refetched: Redis already holds the
	// last three days. Requests never fetch: they only read the Redis snapshot.
	exchangeMaxFallbackDays = 3

	// nbuBaseURL is the NBU exchange endpoint without query parameters.
	nbuBaseURL = "https://bank.gov.ua/NBU_Exchange/exchange"
)

// exchangeWantedCodes lists the ticker currencies in display order.
var exchangeWantedCodes = [3]string{"EUR", "USD", "PLN"}

// exchangeRetryDelays spaces the three retries of the daily fetch.
var exchangeRetryDelays = [3]time.Duration{5 * time.Second, 10 * time.Second, 15 * time.Second}

// ExchangeStore persists dated rate snapshots and serves the newest one.
// The service owns this contract; internal/repository/redis implements it.
type ExchangeStore interface {
	Save(ctx context.Context, date time.Time, rates []*models.ExchangeRate) error
	Latest(ctx context.Context) ([]*models.ExchangeRate, time.Time, error)
}

// HTTPClient is the JSON-over-HTTP contract the exchange refresh needs.
// *httpclient.Client implements it; tests use fakes.
type HTTPClient interface {
	GetJSON(ctx context.Context, url string, out any) error
}

// ExchangeService refreshes NBU rates into Redis and serves them from it.
type ExchangeService struct {
	http        HTTPClient
	store       ExchangeStore
	logger      *slog.Logger
	now         func() time.Time
	retryDelays [3]time.Duration

	refreshMu sync.Mutex
}

// NewExchangeService creates an ExchangeService over http and store.
func NewExchangeService(
	httpClient HTTPClient,
	store ExchangeStore,
	logger *slog.Logger,
) *ExchangeService {
	return &ExchangeService{
		http:        httpClient,
		store:       store,
		logger:      logger,
		now:         time.Now,
		retryDelays: exchangeRetryDelays,
	}
}

// exchangeItem mirrors one entry of the NBU JSON document.
type exchangeItem struct {
	CurrencyCode string  `json:"CurrencyCodeL"`
	Units        int     `json:"Units"`
	Amount       float64 `json:"Amount"`
	StartDate    string  `json:"StartDate"`
}

// StartAutoRefresh refreshes rates every interval until ctx is done.
func (s *ExchangeService) StartAutoRefresh(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.Refresh(ctx); err != nil {
				s.logger.ErrorContext(ctx, "failed to refresh exchange rates", "fc", "exchangeService_StartAutoRefresh", "error", err)
			}
		}
	}
}

// Refresh fetches today with retries, then walks back single attempts,
// and stores the first complete day. Only the background job calls it.
func (s *ExchangeService) Refresh(ctx context.Context) error {
	const fc = "exchangeService_Refresh"

	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	now := s.now()

	rates, err := s.fetchWithRetry(ctx, now)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to fetch exchange rates", "fc", fc, "date", now.Format("2006-01-02"), "error", err)

		return fmt.Errorf("%s: no exchange rates available: %w", fc, errs.ErrServiceUnavailable)
	}

	selected := selectRates(rates)
	if selected == nil {
		s.logger.ErrorContext(ctx, "exchange rates miss wanted currencies", "fc", fc, "date", now.Format("2006-01-02"))

		return fmt.Errorf("%s: no exchange rates available: %w", fc, errs.ErrServiceUnavailable)
	}

	if err := s.store.Save(ctx, now, selected); err != nil {
		s.logger.ErrorContext(ctx, "failed to store exchange rates", "fc", fc, "date", now.Format("2006-01-02"), "error", err)

		return fmt.Errorf("%s: save exchange rates: %w", fc, errs.ErrServiceUnavailable)
	}

	return nil
}

// fetchWithRetry tries date once, then retries with growing pauses.
func (s *ExchangeService) fetchWithRetry(ctx context.Context, date time.Time) ([]*models.ExchangeRate, error) {
	const fc = "exchangeService_fetchWithRetry"

	rates, err := s.fetchDay(ctx, date)
	if err == nil {
		return rates, nil
	}

	for i, pause := range s.retryDelays {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("%s: %w", fc, ctx.Err())
		case <-time.After(pause):
		}

		rates, err = s.fetchDay(ctx, date)
		if err == nil {
			return rates, nil
		}

		s.logger.ErrorContext(ctx, "exchange fetch retry failed", "fc", fc, "attempt", i+2, "error", err)
	}

	return nil, fmt.Errorf("%s: %w", fc, err)
}

// fetchDay returns per-unit UAH rates for date (Amount divided by Units).
func (s *ExchangeService) fetchDay(ctx context.Context, date time.Time) ([]*models.ExchangeRate, error) {
	const fc = "exchangeService_fetchDay"

	var items []exchangeItem
	url := nbuBaseURL + "?json&date=" + date.Format("02.01.2006")
	if err := s.http.GetJSON(ctx, url, &items); err != nil {
		return nil, fmt.Errorf("%s: fetch nbu rates: %w", fc, err)
	}

	rates := make([]*models.ExchangeRate, 0, len(items))
	for _, item := range items {
		if item.Units <= 0 {
			continue
		}

		rates = append(rates, &models.ExchangeRate{
			Code: item.CurrencyCode,
			Rate: item.Amount / float64(item.Units),
			Date: item.StartDate,
		})
	}

	return rates, nil
}

// GetRates returns the newest Redis snapshot inside the three-day window.
// It never fetches from NBU: a missing or expired snapshot is a 503.
func (s *ExchangeService) GetRates(ctx context.Context) ([]*models.ExchangeRate, error) {
	const fc = "exchangeService_GetRates"

	rates, date, err := s.store.Latest(ctx)
	if err != nil {
		if errors.Is(err, errs.ErrExchangeRatesNotFound) {
			s.logger.WarnContext(ctx, "no exchange rates stored", "fc", fc)
		} else {
			s.logger.ErrorContext(ctx, "failed to read exchange rates", "fc", fc, "error", err)
		}

		return nil, fmt.Errorf("%s: %w", fc, errs.ErrServiceUnavailable)
	}

	// Calendar-day comparison in YYYY-MM-DD form: immune to time-of-day
	// and server-timezone offsets on either side.
	oldest := s.now().AddDate(0, 0, -exchangeMaxFallbackDays).Format("2006-01-02")
	if date.Format("2006-01-02") < oldest {
		return nil, fmt.Errorf("%s: exchange rates expired: %w", fc, errs.ErrServiceUnavailable)
	}

	return rates, nil
}

// selectRates picks the wanted currencies in display order.
// It returns nil unless every wanted currency is present.
func selectRates(rates []*models.ExchangeRate) []*models.ExchangeRate {
	byCode := make(map[string]*models.ExchangeRate, len(rates))
	for _, rate := range rates {
		byCode[rate.Code] = rate
	}

	selected := make([]*models.ExchangeRate, 0, len(exchangeWantedCodes))
	for _, code := range exchangeWantedCodes {
		rate, ok := byCode[code]
		if !ok {
			return nil
		}

		selected = append(selected, rate)
	}

	return selected
}
