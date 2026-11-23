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

// ContactService is the contact behavior used by the contact handlers.
type ContactService interface {
	List(ctx context.Context) ([]*models.Contact, error)
	Create(ctx context.Context, input service.ContactInput) (*models.Contact, error)
	Update(ctx context.Context, id uuid.UUID, input service.ContactInput) (*models.Contact, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Move(ctx context.Context, id uuid.UUID, direction string) error
}

// ContactHandler exposes the public contacts list and the contact administration endpoints.
type ContactHandler struct {
	svc         ContactService
	middlewares []gin.HandlerFunc
}

// NewContactHandler creates a ContactHandler over svc; middlewares protect the admin routes only.
func NewContactHandler(svc ContactService, middlewares ...gin.HandlerFunc) *ContactHandler {
	return &ContactHandler{svc: svc, middlewares: middlewares}
}

// Register binds the public contacts route and the protected admin group.
func (h *ContactHandler) Register(r *gin.RouterGroup) {
	r.GET(routes.PublicContacts, h.contactPublicList)

	contacts := r.Group(routes.AdminContacts)
	contacts.Use(h.middlewares...)
	contacts.GET("", h.contactPublicList)
	contacts.POST("", h.Create)
	contacts.PUT("/:id", h.Update)
	contacts.DELETE("/:id", h.Delete)
	contacts.POST("/:id/move", h.Move)
}

// contactPublicList handles GET /api/contacts and GET /api/admin/contacts: the ordered
// contacts, where an empty list is [] rather than null.
func (h *ContactHandler) contactPublicList(c *gin.Context) {
	contacts, err := h.svc.List(c.Request.Context())
	if err != nil {
		httptransport.WriteError(c, err)

		return
	}

	items := make([]*dto.ContactResponse, 0, len(contacts))
	for _, contact := range contacts {
		items = append(items, dto.ContactToResponse(contact))
	}

	c.JSON(http.StatusOK, dto.ContactListResponse{Items: items})
}

// Create handles POST /api/admin/contacts with {"type","value","label"}.
func (h *ContactHandler) Create(c *gin.Context) {
	var req dto.ContactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httptransport.WriteError(c, bindError(err))

		return
	}

	contact, err := h.svc.Create(c.Request.Context(), service.ContactInput(req))
	if err != nil {
		httptransport.WriteError(c, err)

		return
	}

	c.JSON(http.StatusCreated, dto.ContactToResponse(contact))
}

// Update handles PUT /api/admin/contacts/:id with {"type","value","label"}.
func (h *ContactHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httptransport.WriteError(c, errs.ErrInvalidInput)

		return
	}

	var req dto.ContactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httptransport.WriteError(c, bindError(err))

		return
	}

	contact, err := h.svc.Update(c.Request.Context(), id, service.ContactInput(req))
	if err != nil {
		httptransport.WriteError(c, err)

		return
	}

	c.JSON(http.StatusOK, dto.ContactToResponse(contact))
}

// Delete handles DELETE /api/admin/contacts/:id.
func (h *ContactHandler) Delete(c *gin.Context) {
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

// Move handles POST /api/admin/contacts/:id/move with {"direction":"up"|"down"}.
func (h *ContactHandler) Move(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httptransport.WriteError(c, errs.ErrInvalidInput)

		return
	}

	var req dto.ContactMoveRequest
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
