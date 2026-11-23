package dto

import (
	"time"

	"github.com/maxaixs/mandat/backend/internal/models"
)

// TelegramChannelResponse is the public Telegram channel stats.
type TelegramChannelResponse struct {
	Subscribers int       `json:"subscribers"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TelegramChannelToResponse maps the channel stats onto their API shape.
func TelegramChannelToResponse(stats *models.ChannelStats) *TelegramChannelResponse {
	return &TelegramChannelResponse{Subscribers: stats.Subscribers, UpdatedAt: stats.UpdatedAt}
}
