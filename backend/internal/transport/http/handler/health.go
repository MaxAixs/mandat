package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/maxaixs/mandat/backend/internal/service"
	"github.com/maxaixs/mandat/backend/internal/transport/http/dto"
)

// HealthHandler exposes liveness and readiness probes.
type HealthHandler struct {
	svc *service.HealthService
}

// NewHealthHandler creates a HealthHandler over svc.
func NewHealthHandler(svc *service.HealthService) *HealthHandler {
	return &HealthHandler{svc: svc}
}

// Register binds the health check routes.
func (h *HealthHandler) Register(r *gin.RouterGroup) {
	health := r.Group("/health")
	health.GET("/live", h.Live)
	health.GET("/ready", h.Ready)
}

// Live handles GET /health/live without checking external dependencies.
func (h *HealthHandler) Live(c *gin.Context) {
	c.JSON(http.StatusOK, dto.HealthResponse{Status: "ok"})
}

// Ready handles GET /health/ready and checks required dependencies.
func (h *HealthHandler) Ready(c *gin.Context) {
	if err := h.svc.Ready(c.Request.Context()); err != nil {
		WriteError(c, err)

		return
	}

	c.JSON(http.StatusOK, dto.HealthResponse{Status: "ok"})
}
