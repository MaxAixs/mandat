package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/maxaixs/mandat/backend/internal/models"
	"github.com/maxaixs/mandat/backend/internal/transport/http"
	"github.com/maxaixs/mandat/backend/internal/transport/http/dto"
	"github.com/maxaixs/mandat/backend/internal/transport/http/routes"
)

// TelegramService is the channel stats behavior used by TelegramHandler.
type TelegramService interface {
	Stats(ctx context.Context) (*models.ChannelStats, error)
}

// TelegramHandler exposes the public Telegram channel stats.
type TelegramHandler struct {
	svc TelegramService
}

// NewTelegramHandler creates a TelegramHandler over svc.
func NewTelegramHandler(svc TelegramService) *TelegramHandler {
	return &TelegramHandler{svc: svc}
}

// Register binds the public Telegram channel route.
func (h *TelegramHandler) Register(r *gin.RouterGroup) {
	r.GET(routes.PublicTelegramChannel, h.Channel)
}

// Channel handles GET /api/telegram/channel.
func (h *TelegramHandler) Channel(c *gin.Context) {
	stats, err := h.svc.Stats(c.Request.Context())
	if err != nil {
		httptransport.WriteError(c, err)

		return
	}

	c.JSON(http.StatusOK, dto.TelegramChannelToResponse(stats))
}
