package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
	"github.com/maxaixs/mandat/backend/internal/service"
	"github.com/maxaixs/mandat/backend/internal/transport/http"
	"github.com/maxaixs/mandat/backend/internal/transport/http/dto"
	"github.com/maxaixs/mandat/backend/internal/transport/http/routes"
)

// AdFormatService is the ad format card behavior used by the ad format handlers.
type AdFormatService interface {
	List(ctx context.Context) ([]*models.AdFormat, error)
	Create(ctx context.Context, input service.AdFormatInput) (*models.AdFormat, error)
	Update(ctx context.Context, id uuid.UUID, input service.AdFormatInput) (*models.AdFormat, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Move(ctx context.Context, id uuid.UUID, direction string) error
}

// AdFormatHandler exposes ad format card administration endpoints.
type AdFormatHandler struct {
	svc         AdFormatService
	middlewares []gin.HandlerFunc
}

// NewAdFormatHandler creates an AdFormatHandler over svc with optional route protection.
func NewAdFormatHandler(svc AdFormatService, middlewares ...gin.HandlerFunc) *AdFormatHandler {
	return &AdFormatHandler{svc: svc, middlewares: middlewares}
}

// Register binds the ad format routes under the admin group.
func (h *AdFormatHandler) Register(r *gin.RouterGroup) {
	formats := r.Group(routes.AdminAdFormats)
	formats.Use(h.middlewares...)
	formats.GET("", h.List)
	formats.POST("", h.Create)
	formats.PUT("/:id", h.Update)
	formats.DELETE("/:id", h.Delete)
	formats.POST("/:id/move", h.Move)
}

// List handles GET /api/admin/ad-formats.
func (h *AdFormatHandler) List(c *gin.Context) {
	writeAdFormatList(c, h.svc)
}

// Create handles POST /api/admin/ad-formats (multipart: title, description, image).
func (h *AdFormatHandler) Create(c *gin.Context) {
	input, err := bindAdFormatInput(c)
	if err != nil {
		httptransport.WriteError(c, err)

		return
	}
	defer closeCover(input.Image)

	format, err := h.svc.Create(c.Request.Context(), input)
	if err != nil {
		httptransport.WriteError(c, err)

		return
	}

	c.JSON(http.StatusCreated, dto.AdFormatToResponse(format))
}

// Update handles PUT /api/admin/ad-formats/:id (multipart: title, description, image, remove_image).
func (h *AdFormatHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httptransport.WriteError(c, errs.ErrInvalidInput)

		return
	}

	input, err := bindAdFormatInput(c)
	if err != nil {
		httptransport.WriteError(c, err)

		return
	}
	defer closeCover(input.Image)

	format, err := h.svc.Update(c.Request.Context(), id, input)
	if err != nil {
		httptransport.WriteError(c, err)

		return
	}

	c.JSON(http.StatusOK, dto.AdFormatToResponse(format))
}

// Delete handles DELETE /api/admin/ad-formats/:id.
func (h *AdFormatHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httptransport.WriteError(c, errs.ErrInvalidInput)

		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		httptransport.WriteError(c, err)

		return
	}

	c.Status(http.StatusOK)
}

// Move handles POST /api/admin/ad-formats/:id/move with {"direction":"up"|"down"}.
func (h *AdFormatHandler) Move(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httptransport.WriteError(c, errs.ErrInvalidInput)

		return
	}

	var req dto.AdFormatMoveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httptransport.WriteError(c, bindError(err))

		return
	}

	if err := h.svc.Move(c.Request.Context(), id, req.Direction); err != nil {
		httptransport.WriteError(c, err)

		return
	}

	c.Status(http.StatusOK)
}

// bindAdFormatInput reads the multipart card form with its optional image part.
func bindAdFormatInput(c *gin.Context) (service.AdFormatInput, error) {
	form, err := c.MultipartForm()
	if err != nil {
		return service.AdFormatInput{}, bindError(err)
	}

	removeImage, err := parseBoolField(formField(form, "remove_image"))
	if err != nil {
		return service.AdFormatInput{}, err
	}

	image, err := openUpload(form, "image")
	if err != nil {
		return service.AdFormatInput{}, err
	}

	return service.AdFormatInput{
		Title:       formField(form, "title"),
		Description: formField(form, "description"),
		Image:       image,
		RemoveImage: removeImage,
	}, nil
}
