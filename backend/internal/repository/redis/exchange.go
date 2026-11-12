package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
	goredis "github.com/redis/go-redis/v9"
)

// Exchange rate persistence in Redis.
const (
	// exchangeKeyPrefix namespaces dated rate snapshots.
	exchangeKeyPrefix = "exchange:rates:"
	// exchangeLatestKey points at the newest snapshot's date (YYYY-MM-DD).
	exchangeLatestKey = "exchange:rates:latest"
	// exchangeRatesTTL expires snapshots after three days without a cleanup job.
	exchangeRatesTTL = 72 * time.Hour
)

// redisCmdable is the Redis surface the exchange store needs.
// *goredis.Client satisfies it; tests use an in-memory fake.
type redisCmdable interface {
	Set(ctx context.Context, key string, value any, expiration time.Duration) *goredis.StatusCmd
	Get(ctx context.Context, key string) *goredis.StringCmd
}

// ExchangeStore persists NBU rate snapshots in Redis.
type ExchangeStore struct {
	client redisCmdable
}

// NewExchangeStore creates an ExchangeStore backed by client.
func NewExchangeStore(client redisCmdable) *ExchangeStore {
	return &ExchangeStore{client: client}
}

// exchangeRecord is the serialized snapshot payload.
type exchangeRecord struct {
	Date  string               `json:"date"`
	Items []exchangeRecordItem `json:"items"`
}

// exchangeRecordItem is one serialized rate.
type exchangeRecordItem struct {
	Code string  `json:"code"`
	Rate float64 `json:"rate"`
}

// Save stores the rates for date and moves the latest pointer onto it.
func (s *ExchangeStore) Save(ctx context.Context, date time.Time, rates []*models.Rate) error {
	const fc = "exchangeStore_Save"

	items := make([]exchangeRecordItem, 0, len(rates))
	for _, rate := range rates {
		if rate == nil {
			continue
		}

		items = append(items, exchangeRecordItem{Code: rate.Code, Rate: rate.Rate})
	}

	payload, err := json.Marshal(exchangeRecord{
		Date:  date.Format("02.01.2006"),
		Items: items,
	})
	if err != nil {
		return fmt.Errorf("%s: encode rates: %w", fc, err)
	}

	dayKey := exchangeKeyPrefix + date.Format("2006-01-02")
	if err := s.client.Set(ctx, dayKey, payload, exchangeRatesTTL).Err(); err != nil {
		return fmt.Errorf("%s: %w", fc, err)
	}

	if err := s.client.Set(ctx, exchangeLatestKey, date.Format("2006-01-02"), exchangeRatesTTL).Err(); err != nil {
		return fmt.Errorf("%s: %w", fc, err)
	}

	return nil
}

// Latest returns the newest snapshot or errs.ErrExchangeRatesNotFound.
func (s *ExchangeStore) Latest(ctx context.Context) ([]*models.Rate, time.Time, error) {
	const fc = "exchangeStore_Latest"

	day, err := s.client.Get(ctx, exchangeLatestKey).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, time.Time{}, fmt.Errorf("%s: %w", fc, errs.ErrExchangeRatesNotFound)
		}

		return nil, time.Time{}, fmt.Errorf("%s: %w", fc, err)
	}

	payload, err := s.client.Get(ctx, exchangeKeyPrefix+day).Bytes()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, time.Time{}, fmt.Errorf("%s: %w", fc, errs.ErrExchangeRatesNotFound)
		}

		return nil, time.Time{}, fmt.Errorf("%s: %w", fc, err)
	}

	var record exchangeRecord
	if err := json.Unmarshal(payload, &record); err != nil {
		return nil, time.Time{}, fmt.Errorf("%s: decode rates: %w", fc, errs.ErrExchangeRatesNotFound)
	}

	date, err := time.ParseInLocation("02.01.2006", record.Date, time.UTC)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("%s: decode rates: %w", fc, errs.ErrExchangeRatesNotFound)
	}

	rates := make([]*models.Rate, 0, len(record.Items))
	for _, item := range record.Items {
		rates = append(rates, &models.Rate{
			Code: item.Code,
			Rate: item.Rate,
			Date: record.Date,
		})
	}

	return rates, date, nil
}
