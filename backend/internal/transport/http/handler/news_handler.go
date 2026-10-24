// Package handler contains the HTTP handlers of the API.
package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/service"
	"github.com/maxaixs/mandat/backend/internal/transport/http/dto"
)

// adminBasePath is the root path of the admin API.
const (
	adminBasePath = "/api/admin"
	newsPath      = "/news"
)

// NewsHandler exposes news administration endpoints.
type NewsHandler struct {
	svc *service.NewsService
}

// NewNewsHandler creates a NewsHandler over svc.
func NewNewsHandler(svc *service.NewsService) *NewsHandler {
	return &NewsHandler{svc: svc}
}

// Register binds the news routes under the admin group.
func (h *NewsHandler) Register(r *gin.RouterGroup) {
	admin := r.Group(adminBasePath)
	{
		news := admin.Group(newsPath)
		news.POST("", h.Create)
		news.GET("", h.List)
		news.GET("/:slug", h.GetBySlug)
		news.PUT("/:slug", h.Update)
		news.DELETE("/:slug", h.Delete)
	}
}

// Create handles POST /api/v1/admin/news.
func (h *NewsHandler) Create(c *gin.Context) {
	var req dto.NewsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		WriteError(c, bindError(err))

		return
	}

	news, err := h.svc.Create(c.Request.Context(), req.ToInput())
	if err != nil {
		WriteError(c, err)

		return
	}

	c.JSON(http.StatusCreated, dto.NewsToResponse(news))
}

// List handles GET /api/v1/admin/news.
func (h *NewsHandler) List(c *gin.Context) {
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

	result, err := h.svc.List(c.Request.Context(), page, limit, c.Query("status"))
	if err != nil {
		WriteError(c, err)

		return
	}

	items := make([]*dto.NewsResponse, 0, len(result.Items))
	for _, news := range result.Items {
		items = append(items, dto.NewsToResponse(news))
	}

	c.JSON(http.StatusOK, dto.ListResponse{
		Items: items,
		Page:  result.Page,
		Limit: result.Limit,
	})
}

// GetBySlug handles GET /api/v1/admin/news/:slug.
func (h *NewsHandler) GetBySlug(c *gin.Context) {
	news, err := h.svc.GetBySlug(c.Request.Context(), c.Param("slug"))
	if err != nil {
		WriteError(c, err)

		return
	}

	c.JSON(http.StatusOK, dto.NewsToResponse(news))
}

// Update handles PUT /api/v1/admin/news/:slug.
func (h *NewsHandler) Update(c *gin.Context) {
	var req dto.NewsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		WriteError(c, bindError(err))
		return
	}

	news, err := h.svc.Update(c.Request.Context(), c.Param("slug"), req.ToInput())
	if err != nil {
		WriteError(c, err)

		return
	}

	c.JSON(http.StatusOK, dto.NewsToResponse(news))
}

// Delete handles DELETE /api/v1/admin/news/:slug.
func (h *NewsHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("slug")); err != nil {
		WriteError(c, err)

		return
	}

	c.Status(http.StatusOK)
}

// queryInt parses a numeric query parameter, falling back to fallback.
func queryInt(c *gin.Context, name string, fallback int) (int, error) {
	raw := c.Query(name)
	if raw == "" {
		return fallback, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, errs.ErrInvalidInput
	}

	return value, nil
}

// bindError classifies a request binding failure.
func bindError(err error) error {
	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		return errs.ErrPayloadTooLarge
	}

	return errs.ErrInvalidInput
}
