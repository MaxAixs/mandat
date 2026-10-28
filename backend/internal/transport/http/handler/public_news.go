package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/maxaixs/mandat/backend/internal/service"
	"github.com/maxaixs/mandat/backend/internal/transport/http/dto"
)

// PublicNewsHandler exposes published news to the public site.
type PublicNewsHandler struct {
	svc *service.NewsService
}

// NewPublicNewsHandler creates a PublicNewsHandler over svc.
func NewPublicNewsHandler(svc *service.NewsService) *PublicNewsHandler {
	return &PublicNewsHandler{svc: svc}
}

// Register binds the public news routes.
func (h *PublicNewsHandler) Register(r *gin.RouterGroup) {
	news := r.Group("/api" + newsPath)
	news.GET("", h.List)
}

// List handles GET /api/news and returns only published articles.
func (h *PublicNewsHandler) List(c *gin.Context) {
	page, err := queryInt(c, "page", 1)
	if err != nil {
		WriteError(c, err)

		return
	}

	limit, err := queryInt(c, "limit", service.DefaultPageLimit)
	if err != nil {
		WriteError(c, err)

		return
	}

	result, err := h.svc.ListPublished(c.Request.Context(), page, limit)
	if err != nil {
		WriteError(c, err)

		return
	}

	items := make([]*dto.PublicNewsResponse, 0, len(result.Items))
	for _, news := range result.Items {
		items = append(items, dto.NewsToPublicResponse(news))
	}

	c.JSON(http.StatusOK, dto.PublicNewsListResponse{
		Items: items,
		Page:  result.Page,
		Limit: result.Limit,
	})
}
