package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/maxaixs/mandat/backend/internal/models"
	"github.com/maxaixs/mandat/backend/internal/transport/http/dto"
)

// RunningLineService is the rate table behavior used by RunningLineHandler.
type RunningLineService interface {
	GetRates(ctx context.Context) ([]*models.Rate, error)
}

// RunningLineHandler exposes the ticker rates: NBU currency and fuel prices.
type RunningLineHandler struct {
	exchange RunningLineService
	fuel     RunningLineService
}

// NewRunningLineHandler creates a RunningLineHandler over the exchange and fuel services.
func NewRunningLineHandler(exchange, fuel RunningLineService) *RunningLineHandler {
	return &RunningLineHandler{exchange: exchange, fuel: fuel}
}

// Register binds the running line routes.
func (h *RunningLineHandler) Register(r *gin.RouterGroup) {
	r.GET("/api/exchange/rates", h.GetExchangeRates)
	r.GET("/api/fuel/rates", h.GetFuelRates)
}

// GetExchangeRates handles GET /api/exchange/rates and returns the rate table.
func (h *RunningLineHandler) GetExchangeRates(c *gin.Context) {
	rates, err := h.exchange.GetRates(c.Request.Context())
	if err != nil {
		WriteError(c, err)

		return
	}

	c.JSON(http.StatusOK, dto.ExchangeToRatesResponse(rates))
}

// GetFuelRates handles GET /api/fuel/rates and returns the price table.
func (h *RunningLineHandler) GetFuelRates(c *gin.Context) {
	rates, err := h.fuel.GetRates(c.Request.Context())
	if err != nil {
		WriteError(c, err)

		return
	}

	c.JSON(http.StatusOK, dto.FuelToRatesResponse(rates))
}
