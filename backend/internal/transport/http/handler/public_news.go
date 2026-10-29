package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/maxaixs/mandat/backend/internal/models"
	"github.com/maxaixs/mandat/backend/internal/service"
	"github.com/maxaixs/mandat/backend/internal/transport/http/dto"
)

// PublicNewsService is the published-news behavior used by PublicNewsHandler.
type PublicNewsService interface {
	GetPublishedBySlug(ctx context.Context, slug string) (*models.News, error)
	ListPublished(ctx context.Context, page int, limit int) (*service.ListResult, error)
}

// PublicNewsHandler exposes published news to the public site.
type PublicNewsHandler struct {
	svc              PublicNewsService
	viewerMiddleware []gin.HandlerFunc
}

// NewPublicNewsHandler creates a PublicNewsHandler over svc.
func NewPublicNewsHandler(
	svc PublicNewsService,
	viewerMiddleware ...gin.HandlerFunc,
) *PublicNewsHandler {
	return &PublicNewsHandler{svc: svc, viewerMiddleware: viewerMiddleware}
}

// Register binds the public news routes.
func (h *PublicNewsHandler) Register(r *gin.RouterGroup) {
	news := r.Group("/api" + newsPath)
	news.GET("", h.List)

	detailHandlers := append(h.viewerMiddleware, h.GetBySlug)
	news.GET("/:slug", detailHandlers...)
}

// GetBySlug handles GET /api/news/:slug and returns one published article.
func (h *PublicNewsHandler) GetBySlug(c *gin.Context) {
	news, err := h.svc.GetPublishedBySlug(c.Request.Context(), c.Param("slug"))
	if err != nil {
		WriteError(c, err)

		return
	}

	c.JSON(http.StatusOK, dto.NewsToPublicDetailResponse(news))
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
