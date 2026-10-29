package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
	"github.com/maxaixs/mandat/backend/internal/transport/http/dto"
)

// Authenticator validates an opaque session token.
type Authenticator interface {
	Authenticate(ctx context.Context, rawToken string) (*models.AdminUser, error)
}

// RequireAuth loads the staff identity carried by the session cookie.
func RequireAuth(auth Authenticator) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(dto.AdminSessionCookie)
		if err != nil {
			abortWithAuthError(c, http.StatusUnauthorized, "Authentication required")

			return
		}

		user, err := auth.Authenticate(c.Request.Context(), token)
		if err != nil {
			status := http.StatusUnauthorized
			message := "Authentication required"
			if !errors.Is(err, errs.ErrUnauthorized) {
				status = http.StatusInternalServerError
				message = "Internal server error"
			}
			abortWithAuthError(c, status, message)

			return
		}

		c.Set(dto.CurrentUserKey, user)
		c.Next()
	}
}

// RequireRoles allows only authenticated users with one of roles.
func RequireRoles(roles ...models.Role) gin.HandlerFunc {
	allowed := make(map[models.Role]struct{}, len(roles))
	for _, role := range roles {
		allowed[role] = struct{}{}
	}

	return func(c *gin.Context) {
		user, ok := CurrentUser(c)
		if !ok {
			abortWithAuthError(c, http.StatusUnauthorized, "Authentication required")
			return
		}
		if _, ok := allowed[user.Role]; !ok {
			abortWithAuthError(c, http.StatusForbidden, "Insufficient permissions")
			return
		}

		c.Next()
	}
}

// CurrentUser returns the staff identity set by RequireAuth.
func CurrentUser(c *gin.Context) (*models.AdminUser, bool) {
	value, ok := c.Get(dto.CurrentUserKey)
	if !ok {
		return nil, false
	}

	user, ok := value.(*models.AdminUser)
	return user, ok && user != nil
}

func abortWithAuthError(c *gin.Context, status int, message string) {
	c.AbortWithStatusJSON(status, gin.H{
		"error": gin.H{"status": status, "message": message},
	})
}
