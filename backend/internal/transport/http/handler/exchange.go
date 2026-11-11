package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/maxaixs/mandat/backend/internal/models"
	"github.com/maxaixs/mandat/backend/internal/transport/http/dto"
)

// ExchangeService is the rate table behavior used by ExchangeHandler.
type ExchangeService interface {
	GetRates(ctx context.Context) ([]*models.ExchangeRate, error)
}

// ExchangeHandler exposes NBU currency rates to the public site.
type ExchangeHandler struct {
	svc ExchangeService
}

// NewExchangeHandler creates an ExchangeHandler over svc.
func NewExchangeHandler(svc ExchangeService) *ExchangeHandler {
	return &ExchangeHandler{svc: svc}
}

// Register binds the exchange routes.
func (h *ExchangeHandler) Register(r *gin.RouterGroup) {
	exchange := r.Group("/api/exchange")
	exchange.GET("/rates", h.GetRates)
}

// GetRates handles GET /api/exchange/rates and returns the rate table.
func (h *ExchangeHandler) GetRates(c *gin.Context) {
	rates, err := h.svc.GetRates(c.Request.Context())
	if err != nil {
		WriteError(c, err)

		return
	}

	c.JSON(http.StatusOK, dto.ExchangeToRatesResponse(rates))
}
