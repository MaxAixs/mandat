package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubTelegramService returns fixed channel stats or a fixed error.
type stubTelegramService struct {
	stats *models.ChannelStats
	err   error
}

func (s *stubTelegramService) Stats(context.Context) (*models.ChannelStats, error) {
	if s.err != nil {
		return nil, s.err
	}

	return s.stats, nil
}

// setupTelegramRouter mounts the channel route on a bare gin engine.
func setupTelegramRouter(svc TelegramService) http.Handler {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	NewTelegramHandler(svc).Register(engine.Group(""))

	return engine
}

func TestTelegramHandler_ChannelReturnsSubscribers(t *testing.T) {
	updatedAt := time.Date(2026, time.November, 23, 14, 0, 0, 0, time.UTC)
	svc := &stubTelegramService{stats: &models.ChannelStats{Subscribers: 3846, UpdatedAt: updatedAt}}
	router := setupTelegramRouter(svc)

	rec := doRequest(t, router, http.MethodGet, "/api/telegram/channel", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var payload struct {
		Subscribers int       `json:"subscribers"`
		UpdatedAt   time.Time `json:"updated_at"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))

	assert.Equal(t, 3846, payload.Subscribers)
	assert.True(t, payload.UpdatedAt.Equal(updatedAt),
		"updated_at must round-trip the stored timestamp, got %v", payload.UpdatedAt)
}

func TestTelegramHandler_ChannelUnavailable(t *testing.T) {
	svc := &stubTelegramService{err: errs.ErrServiceUnavailable}
	router := setupTelegramRouter(svc)

	rec := doRequest(t, router, http.MethodGet, "/api/telegram/channel", "")

	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
}
