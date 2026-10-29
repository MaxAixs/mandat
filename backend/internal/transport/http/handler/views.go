package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/maxaixs/mandat/backend/internal/transport/http/dto"
)

// ViewService records a throttled public article view.
type ViewService interface {
	RecordView(ctx context.Context, slug string, viewerID uuid.UUID) (int64, error)
}

// ViewHandler exposes public article view recording.
type ViewHandler struct {
	svc              ViewService
	viewerMiddleware gin.HandlerFunc
}

// NewViewHandler creates a ViewHandler over svc.
func NewViewHandler(svc ViewService, viewerMiddleware gin.HandlerFunc) *ViewHandler {
	return &ViewHandler{svc: svc, viewerMiddleware: viewerMiddleware}
}

// Register binds the public view route.
func (h *ViewHandler) Register(r *gin.RouterGroup) {
	news := r.Group("/api" + newsPath)
	news.POST("/:slug/view", h.viewerMiddleware, h.Record)
}

// Record handles POST /api/news/:slug/view.
func (h *ViewHandler) Record(c *gin.Context) {
	viewerID, ok := c.Get(dto.ViewerIDKey)
	if !ok {
		c.AbortWithStatus(http.StatusInternalServerError)

		return
	}

	anonymousID, ok := viewerID.(uuid.UUID)
	if !ok || anonymousID == uuid.Nil {
		c.AbortWithStatus(http.StatusInternalServerError)

		return
	}

	views, err := h.svc.RecordView(c.Request.Context(), c.Param("slug"), anonymousID)
	if err != nil {
		WriteError(c, err)

		return
	}

	c.JSON(http.StatusOK, dto.ViewResponse{Views: views})
}
