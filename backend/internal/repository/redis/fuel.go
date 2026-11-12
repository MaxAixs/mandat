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

// Fuel price persistence in Redis.
const (
	// fuelKeyPrefix namespaces dated price snapshots.
	fuelKeyPrefix = "fuel:rates:"
	// fuelLatestKey points at the newest snapshot's date (YYYY-MM-DD).
	fuelLatestKey = "fuel:latest"
	// fuelRatesTTL expires snapshots after three days without a cleanup job.
	fuelRatesTTL = 72 * time.Hour
)

// FuelStore persists fuel price snapshots in Redis.
type FuelStore struct {
	client redisCmdable
}

// NewFuelStore creates a FuelStore backed by client.
func NewFuelStore(client redisCmdable) *FuelStore {
	return &FuelStore{client: client}
}

// fuelRecord is the serialized snapshot payload.
type fuelRecord struct {
	Date  string           `json:"date"`
	Items []fuelRecordItem `json:"items"`
}

// fuelRecordItem is one serialized price.
type fuelRecordItem struct {
	Code string  `json:"code"`
	Rate float64 `json:"rate"`
}

// Save stores the prices for date and moves the latest pointer onto it.
func (s *FuelStore) Save(ctx context.Context, date time.Time, rates []*models.Rate) error {
	const fc = "fuelStore_Save"

	items := make([]fuelRecordItem, 0, len(rates))
	for _, rate := range rates {
		if rate == nil {
			continue
		}

		items = append(items, fuelRecordItem{Code: rate.Code, Rate: rate.Rate})
	}

	payload, err := json.Marshal(fuelRecord{
		Date:  date.Format("02.01.2006"),
		Items: items,
	})
	if err != nil {
		return fmt.Errorf("%s: encode rates: %w", fc, err)
	}

	dayKey := fuelKeyPrefix + date.Format("2006-01-02")
	if err := s.client.Set(ctx, dayKey, payload, fuelRatesTTL).Err(); err != nil {
		return fmt.Errorf("%s: %w", fc, err)
	}

	if err := s.client.Set(ctx, fuelLatestKey, date.Format("2006-01-02"), fuelRatesTTL).Err(); err != nil {
		return fmt.Errorf("%s: %w", fc, err)
	}

	return nil
}

// Latest returns the newest snapshot or errs.ErrFuelRatesNotFound.
func (s *FuelStore) Latest(ctx context.Context) ([]*models.Rate, time.Time, error) {
	const fc = "fuelStore_Latest"

	day, err := s.client.Get(ctx, fuelLatestKey).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, time.Time{}, fmt.Errorf("%s: %w", fc, errs.ErrFuelRatesNotFound)
		}

		return nil, time.Time{}, fmt.Errorf("%s: %w", fc, err)
	}

	payload, err := s.client.Get(ctx, fuelKeyPrefix+day).Bytes()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, time.Time{}, fmt.Errorf("%s: %w", fc, errs.ErrFuelRatesNotFound)
		}

		return nil, time.Time{}, fmt.Errorf("%s: %w", fc, err)
	}

	var record fuelRecord
	if err := json.Unmarshal(payload, &record); err != nil {
		return nil, time.Time{}, fmt.Errorf("%s: decode rates: %w", fc, errs.ErrFuelRatesNotFound)
	}

	date, err := time.ParseInLocation("02.01.2006", record.Date, time.UTC)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("%s: decode rates: %w", fc, errs.ErrFuelRatesNotFound)
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
