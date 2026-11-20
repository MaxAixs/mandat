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

// PublicAdFormatService is the read-only card behavior used by PublicAdFormatHandler.
type PublicAdFormatService interface {
	List(ctx context.Context) ([]*models.AdFormat, error)
}

// PublicAdFormatHandler exposes the cards of the advertising page.
type PublicAdFormatHandler struct {
	svc PublicAdFormatService
}

// NewPublicAdFormatHandler creates a PublicAdFormatHandler over svc.
func NewPublicAdFormatHandler(svc PublicAdFormatService) *PublicAdFormatHandler {
	return &PublicAdFormatHandler{svc: svc}
}

// Register binds the public ad format route.
func (h *PublicAdFormatHandler) Register(r *gin.RouterGroup) {
	r.GET(routes.PublicAdFormats, h.List)
}

// List handles GET /api/ad-formats.
func (h *PublicAdFormatHandler) List(c *gin.Context) {
	writeAdFormatList(c, h.svc)
}

// writeAdFormatList responds with the ordered cards; an empty list is [] rather than null.
func writeAdFormatList(c *gin.Context, svc PublicAdFormatService) {
	formats, err := svc.List(c.Request.Context())
	if err != nil {
		httptransport.WriteError(c, err)

		return
	}

	items := make([]*dto.AdFormatResponse, 0, len(formats))
	for _, format := range formats {
		items = append(items, dto.AdFormatToResponse(format))
	}

	c.JSON(http.StatusOK, dto.AdFormatListResponse{Items: items})
}
