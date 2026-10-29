package models

import (
	"time"

	"github.com/google/uuid"
)

// Role is a staff member's authorization role.
type Role string

const (
	RoleAdmin     Role = "admin"
	RoleModerator Role = "moderator"
)

// Valid reports whether r is a known staff role.
func (r Role) Valid() bool {
	return r == RoleAdmin || r == RoleModerator
}

// AdminSession is a revocable server-side authentication session.
type AdminSession struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash []byte
	CreatedAt time.Time
	ExpiresAt time.Time
}

// AdminUser is a staff account provisioned through SQL.
type AdminUser struct {
	ID           uuid.UUID
	Login        string
	PasswordHash string
	Role         Role
}
