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
	"github.com/maxaixs/mandat/backend/pkg/httpclient"
)

// Fuel refresh policy mirrors exchange: today gets one attempt plus
// three retries with growing pauses. Requests never fetch: they only
// read the Redis snapshot.
const (
	fuelMaxFallbackDays = 3
)

// fuelCode maps display codes onto averages keys in ticker order.
// A98 shows the a95_plus (premium 95) value; this mapping is a product
// decision, not an NBU-style code equivalence.
var fuelCodes = [5]struct{ Code, Key string }{
	{Code: "A95", Key: "a95"},
	{Code: "A92", Key: "a92"},
	{Code: "A98", Key: "a95_plus"},
	{Code: "DIESEL", Key: "diesel"},
	{Code: "LPG", Key: "lpg"},
}

// fuelPayload mirrors the fuel API document.
type fuelPayload struct {
	Meta struct {
		UpdatedAt string `json:"updated_at"`
	} `json:"meta"`
	Averages map[string]*float64 `json:"averages"`
}

// FuelStore persists dated fuel snapshots and serves the newest one.
// The service owns this contract; internal/repository/redis implements it.
type FuelStore interface {
	Save(ctx context.Context, date time.Time, rates []*models.Rate) error
	Latest(ctx context.Context) ([]*models.Rate, time.Time, error)
}

// FuelService refreshes fuel prices into Redis and serves them from it.
type FuelService struct {
	http        HTTPClient
	store       FuelStore
	logger      *slog.Logger
	now         func() time.Time
	retryDelays [3]time.Duration
	apiURL      string

	refreshMu sync.Mutex
}

// NewFuelService creates a FuelService over http and store.
func NewFuelService(httpClient HTTPClient, store FuelStore, logger *slog.Logger, apiURL string) *FuelService {
	return &FuelService{
		http:        httpClient,
		store:       store,
		logger:      logger,
		now:         time.Now,
		retryDelays: httpclient.DefaultDelays,
		apiURL:      apiURL,
	}
}

// StartAutoRefresh refreshes rates every interval until ctx is done.
func (s *FuelService) StartAutoRefresh(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.Refresh(ctx); err != nil {
				s.logger.ErrorContext(ctx, "failed to refresh fuel rates", "fc", "fuelService_StartAutoRefresh", "error", err)
			}
		}
	}
}

// Refresh fetches today's fuel averages with retries and stores a
// complete day. Only the background job calls it.
func (s *FuelService) Refresh(ctx context.Context) error {
	const fc = "fuelService_Refresh"

	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	rates, date, err := s.fetchWithRetry(ctx)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to fetch fuel rates", "fc", fc, "error", err)

		return fmt.Errorf("%s: no fuel rates available: %w", fc, errs.ErrServiceUnavailable)
	}

	if err := s.store.Save(ctx, date, rates); err != nil {
		s.logger.ErrorContext(ctx, "failed to store fuel rates", "fc", fc, "date", date.Format("2006-01-02"), "error", err)

		return fmt.Errorf("%s: save fuel rates: %w", fc, errs.ErrServiceUnavailable)
	}

	return nil
}

// fetchWithRetry tries the fuel endpoint once, then retries with pauses.
func (s *FuelService) fetchWithRetry(ctx context.Context) ([]*models.Rate, time.Time, error) {
	const fc = "fuelService_fetchWithRetry"

	var (
		rates []*models.Rate
		day   time.Time
	)

	fetchErr := s.http.FetchWithRetries(
		ctx,
		s.retryDelays,
		"fuel fetch retry failed",
		func(fnCtx context.Context) error {
			var err error
			rates, day, err = s.fetchDay(fnCtx)

			return err
		},
	)
	if fetchErr != nil {
		return nil, time.Time{}, fmt.Errorf("%s: %w", fc, fetchErr)
	}

	return rates, day, nil
}

// fetchDay returns today's fuel averages in ticker order.
// A null or missing average rejects the whole day.
func (s *FuelService) fetchDay(ctx context.Context) ([]*models.Rate, time.Time, error) {
	const fc = "fuelService_fetchDay"

	var payload fuelPayload
	if err := s.http.GetJSON(ctx, s.apiURL, &payload); err != nil {
		return nil, time.Time{}, fmt.Errorf("%s: fetch fuel rates: %w", fc, err)
	}

	updatedAt, err := time.Parse(time.RFC3339, payload.Meta.UpdatedAt)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("%s: decode fuel rates: %w", fc, err)
	}

	rates := make([]*models.Rate, 0, len(fuelCodes))
	for _, code := range fuelCodes {
		rate, ok := payload.Averages[code.Key]
		if !ok || rate == nil {
			return nil, time.Time{}, fmt.Errorf("%s: miss fuel rate %s: %w", fc, code.Key, errs.ErrServiceUnavailable)
		}

		rates = append(rates, &models.Rate{
			Code: code.Code,
			Rate: *rate,
			Date: updatedAt.Format("02.01.2006"),
		})
	}

	return rates, updatedAt, nil
}

// GetRates returns the newest Redis snapshot inside the three-day window.
// It never fetches: a missing or expired snapshot is a 503.
func (s *FuelService) GetRates(ctx context.Context) ([]*models.Rate, error) {
	const fc = "fuelService_GetRates"

	rates, date, err := s.store.Latest(ctx)
	if err != nil {
		if errors.Is(err, errs.ErrFuelRatesNotFound) {
			s.logger.WarnContext(ctx, "no fuel rates stored", "fc", fc)
		} else {
			s.logger.ErrorContext(ctx, "failed to read fuel rates", "fc", fc, "error", err)
		}

		return nil, fmt.Errorf("%s: %w", fc, errs.ErrServiceUnavailable)
	}

	oldest := s.now().AddDate(0, 0, -fuelMaxFallbackDays).Format("2006-01-02")
	if date.Format("2006-01-02") < oldest {
		return nil, fmt.Errorf("%s: fuel rates expired: %w", fc, errs.ErrServiceUnavailable)
	}

	return rates, nil
}
