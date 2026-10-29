package dto

import (
	"github.com/google/uuid"
	"github.com/maxaixs/mandat/backend/internal/models"
)

const (
	// AdminSessionCookie is the browser cookie carrying the opaque session token.
	AdminSessionCookie = "mandat_admin_session"
	// CurrentUserKey is the Gin context key set by authentication middleware.
	CurrentUserKey = "authenticatedAdminUser"
)

// LoginRequest is the staff login payload.
type LoginRequest struct {
	Login    string `json:"login" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// AuthUserResponse is the safe public representation of a staff account.
type AuthUserResponse struct {
	ID    uuid.UUID `json:"id"`
	Login string    `json:"login"`
	Role  string    `json:"role"`
}

// AdminUserToResponse maps a staff account without credential material.
func AdminUserToResponse(user *models.AdminUser) *AuthUserResponse {
	if user == nil {
		return nil
	}

	return &AuthUserResponse{
		ID:    user.ID,
		Login: user.Login,
		Role:  string(user.Role),
	}
}
