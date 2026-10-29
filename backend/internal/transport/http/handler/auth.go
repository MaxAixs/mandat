package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/maxaixs/mandat/backend/internal/models"
	"github.com/maxaixs/mandat/backend/internal/service"
	"github.com/maxaixs/mandat/backend/internal/transport/http/dto"
)

const authBasePath = "/api/auth"

// AuthService is the authentication behavior used by AuthHandler.
type AuthService interface {
	Login(ctx context.Context, login string, password string) (*service.AuthResult, error)
	Authenticate(ctx context.Context, rawToken string) (*models.AdminUser, error)
	Logout(ctx context.Context, rawToken string) error
}

// AuthHandler exposes staff login, logout, and current-user endpoints.
type AuthHandler struct {
	svc            AuthService
	secureCookie   bool
	authMiddleware gin.HandlerFunc
	loginRateLimit gin.HandlerFunc
}

// NewAuthHandler creates an AuthHandler over svc.
func NewAuthHandler(
	svc AuthService,
	secureCookie bool,
	authMiddleware gin.HandlerFunc,
	loginRateLimit gin.HandlerFunc,
) *AuthHandler {
	return &AuthHandler{
		svc:            svc,
		secureCookie:   secureCookie,
		authMiddleware: authMiddleware,
		loginRateLimit: loginRateLimit,
	}
}

// Register binds public login and authenticated session routes.
func (h *AuthHandler) Register(r *gin.RouterGroup) {
	auth := r.Group(authBasePath)
	auth.POST("/login", h.loginRateLimit, h.Login)

	authenticated := auth.Group("")
	authenticated.Use(h.authMiddleware)
	authenticated.POST("/logout", h.Logout)
	authenticated.GET("/me", h.Me)
}

// Login handles POST /api/auth/login.
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		WriteError(c, bindError(err))

		return
	}

	result, err := h.svc.Login(c.Request.Context(), req.Login, req.Password)
	if err != nil {
		WriteError(c, err)

		return
	}

	h.setSessionCookie(c, result.Token, result.ExpiresAt)
	c.JSON(http.StatusOK, dto.AdminUserToResponse(result.User))
}

// Logout handles POST /api/auth/logout.
func (h *AuthHandler) Logout(c *gin.Context) {
	token, err := c.Cookie(dto.AdminSessionCookie)
	if err != nil {
		WriteError(c, err)

		return
	}

	if err := h.svc.Logout(c.Request.Context(), token); err != nil {
		WriteError(c, err)

		return
	}

	h.clearSessionCookie(c)
	c.Status(http.StatusNoContent)
}

// Me handles GET /api/auth/me.
func (h *AuthHandler) Me(c *gin.Context) {
	value, ok := c.Get(dto.CurrentUserKey)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)

		return
	}

	user, ok := value.(*models.AdminUser)
	if !ok || user == nil {
		c.AbortWithStatus(http.StatusUnauthorized)

		return
	}

	c.JSON(http.StatusOK, dto.AdminUserToResponse(user))
}

func (h *AuthHandler) setSessionCookie(c *gin.Context, token string, expiresAt time.Time) {
	maxAge := max(int(time.Until(expiresAt).Seconds()), 0)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		dto.AdminSessionCookie,
		token,
		maxAge,
		"/",
		"",
		h.secureCookie,
		true,
	)
}

func (h *AuthHandler) clearSessionCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(dto.AdminSessionCookie, "", -1, "/", "", h.secureCookie, true)
}
