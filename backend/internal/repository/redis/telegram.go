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

// telegramChannelKey holds the latest channel stats; it has no TTL, so the last
// known value survives restarts and failed refreshes.
const telegramChannelKey = "telegram:channel"

// TelegramStore persists the Telegram channel stats in Redis.
type TelegramStore struct {
	client redisCmdable
}

// NewTelegramStore creates a TelegramStore backed by client.
func NewTelegramStore(client redisCmdable) *TelegramStore {
	return &TelegramStore{client: client}
}

// telegramRecord is the serialized stats payload.
type telegramRecord struct {
	Subscribers int       `json:"subscribers"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Save replaces the stored stats.
func (s *TelegramStore) Save(ctx context.Context, stats *models.ChannelStats) error {
	const fc = "telegramStore_Save"

	payload, err := json.Marshal(telegramRecord{Subscribers: stats.Subscribers, UpdatedAt: stats.UpdatedAt})
	if err != nil {
		return fmt.Errorf("%s: encode stats: %w", fc, err)
	}

	if err := s.client.Set(ctx, telegramChannelKey, payload, 0).Err(); err != nil {
		return fmt.Errorf("%s: %w", fc, err)
	}

	return nil
}

// Latest returns the stored stats or errs.ErrTelegramStatsNotFound.
func (s *TelegramStore) Latest(ctx context.Context) (*models.ChannelStats, error) {
	const fc = "telegramStore_Latest"

	payload, err := s.client.Get(ctx, telegramChannelKey).Bytes()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, fmt.Errorf("%s: %w", fc, errs.ErrTelegramStatsNotFound)
		}

		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	var record telegramRecord
	if err := json.Unmarshal(payload, &record); err != nil {
		return nil, fmt.Errorf("%s: decode stats: %w", fc, errs.ErrTelegramStatsNotFound)
	}

	return &models.ChannelStats{Subscribers: record.Subscribers, UpdatedAt: record.UpdatedAt}, nil
}
