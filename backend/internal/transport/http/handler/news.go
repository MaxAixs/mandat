// Package handler contains the HTTP handlers of the API.
package handler

import (
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
	"github.com/maxaixs/mandat/backend/internal/service"
	"github.com/maxaixs/mandat/backend/internal/transport/http/dto"
	"github.com/maxaixs/mandat/backend/pkg/storage/r2"
)

// adminBasePath is the root path of the admin API.
const (
	adminBasePath = "/api/admin"
	newsPath      = "/news"
)

// AdminNewsPrefix is the matched route prefix of the news admin endpoints.
// The body-size middleware uses it to widen the cap for cover uploads.
const AdminNewsPrefix = adminBasePath + newsPath

// NewsService is the news administration behavior used by NewsHandler.
type NewsService interface {
	Create(ctx context.Context, input service.NewsInput) (*models.News, error)
	GetBySlug(ctx context.Context, slug string) (*models.News, error)
	List(ctx context.Context, page int, limit int, status string, category string) (*service.ListResult, error)
	Update(ctx context.Context, slug string, input service.NewsInput) (*models.News, error)
	Delete(ctx context.Context, slug string) error
}

// NewsHandler exposes news administration endpoints.
type NewsHandler struct {
	svc         NewsService
	middlewares []gin.HandlerFunc
}

// NewNewsHandler creates a NewsHandler over svc with optional route protection.
func NewNewsHandler(svc NewsService, middlewares ...gin.HandlerFunc) *NewsHandler {
	return &NewsHandler{svc: svc, middlewares: middlewares}
}

// Register binds the news routes under the admin group.
func (h *NewsHandler) Register(r *gin.RouterGroup) {
	admin := r.Group(adminBasePath)
	admin.Use(h.middlewares...)
	{
		news := admin.Group(newsPath)
		news.POST("", h.Create)
		news.GET("", h.List)
		news.GET("/:slug", h.GetBySlug)
		news.PUT("/:slug", h.Update)
		news.DELETE("/:slug", h.Delete)
	}
}

// Create handles POST /api/admin/news.
func (h *NewsHandler) Create(c *gin.Context) {
	input, err := h.bindInput(c)
	if err != nil {
		WriteError(c, err)

		return
	}
	defer closeCover(input.Cover)

	news, err := h.svc.Create(c.Request.Context(), input)
	if err != nil {
		WriteError(c, err)

		return
	}

	c.JSON(http.StatusCreated, dto.NewsToResponse(news))
}

// List handles GET /api/admin/news.
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

	result, err := h.svc.List(c.Request.Context(), page, limit, c.Query("status"), c.Query("category"))
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

// GetBySlug handles GET /api/admin/news/:slug.
func (h *NewsHandler) GetBySlug(c *gin.Context) {
	news, err := h.svc.GetBySlug(c.Request.Context(), c.Param("slug"))
	if err != nil {
		WriteError(c, err)

		return
	}

	c.JSON(http.StatusOK, dto.NewsToResponse(news))
}

// Update handles PUT /api/admin/news/:slug.
func (h *NewsHandler) Update(c *gin.Context) {
	input, err := h.bindInput(c)
	if err != nil {
		WriteError(c, err)
		return
	}
	defer closeCover(input.Cover)

	news, err := h.svc.Update(c.Request.Context(), c.Param("slug"), input)
	if err != nil {
		WriteError(c, err)

		return
	}

	c.JSON(http.StatusOK, dto.NewsToResponse(news))
}

// Delete handles DELETE /api/admin/news/:slug.
func (h *NewsHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("slug")); err != nil {
		WriteError(c, err)

		return
	}

	c.Status(http.StatusOK)
}

// bindInput decodes JSON bodies as before and accepts multipart forms
// carrying the same fields plus an optional cover file part.
func (h *NewsHandler) bindInput(c *gin.Context) (service.NewsInput, error) {
	if !isMultipart(c) {
		var req dto.NewsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			return service.NewsInput{}, bindError(err)
		}

		return req.ToInput(), nil
	}

	form, err := c.MultipartForm()
	if err != nil {
		return service.NewsInput{}, bindError(err)
	}

	req := dto.NewsRequest{
		Category:    formField(form, "category"),
		Title:       formField(form, "title"),
		Description: formField(form, "description"),
		Text:        formField(form, "text"),
		Language:    formField(form, "language"),
		Status:      formField(form, "status"),
		ImageUrl:    formField(form, "image_url"),
		Source:      formField(form, "source"),
		SourceURL:   formField(form, "source_url"),
	}

	featured, err := parseBoolField(formField(form, "featured"))
	if err != nil {
		return service.NewsInput{}, err
	}
	req.Featured = featured

	mainEvent, err := parseBoolField(formField(form, "main_event"))
	if err != nil {
		return service.NewsInput{}, err
	}
	req.MainEvent = mainEvent

	input := req.ToInput()

	cover, err := openCover(form)
	if err != nil {
		return service.NewsInput{}, err
	}
	input.Cover = cover

	return input, nil
}

// isMultipart reports multipart form submissions.
// Media types are case-insensitive, so the header is lowered first.
func isMultipart(c *gin.Context) bool {
	return strings.HasPrefix(strings.ToLower(c.GetHeader("Content-Type")), "multipart/form-data")
}

// formField returns the first value of a multipart text field.
func formField(form *multipart.Form, name string) string {
	values := form.Value[name]
	if len(values) == 0 {
		return ""
	}

	return values[0]
}

// parseBoolField reads a form checkbox value.
// Empty means unchecked; anything else must parse or the input is invalid.
func parseBoolField(raw string) (bool, error) {
	if raw == "" {
		return false, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, errs.ErrInvalidInput
	}

	return value, nil
}

// openCover extracts the optional cover file part. A missing part is not
// an error; the caller closes the returned body after the service call.
// Oversized files fail here, before the service reads them.
func openCover(form *multipart.Form) (*service.CoverUpload, error) {
	files := form.File["cover"]
	if len(files) == 0 {
		return nil, nil
	}

	fh := files[0]
	if fh.Size > r2.MaxUploadBytes {
		return nil, errs.ErrPayloadTooLarge
	}

	f, err := fh.Open()
	if err != nil {
		return nil, errs.ErrInvalidInput
	}

	return &service.CoverUpload{
		Body:        f,
		Filename:    fh.Filename,
		ContentType: fh.Header.Get("Content-Type"),
		Size:        fh.Size,
	}, nil
}

// closeCover releases the uploaded cover file after the service call.
func closeCover(cover *service.CoverUpload) {
	if cover == nil || cover.Body == nil {
		return
	}
	if closer, ok := cover.Body.(io.Closer); ok {
		_ = closer.Close()
	}
}

// bindError classifies a request binding failure.
func bindError(err error) error {
	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		return errs.ErrPayloadTooLarge
	}

	return errs.ErrInvalidInput
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
