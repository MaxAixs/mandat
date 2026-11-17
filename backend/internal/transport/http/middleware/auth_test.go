package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
	"github.com/maxaixs/mandat/backend/internal/transport/http/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// authFakeAuthenticator stands in for the session service.
type authFakeAuthenticator struct {
	user *models.AdminUser
	err  error
}

func (f *authFakeAuthenticator) Authenticate(
	_ context.Context,
	_ string,
) (*models.AdminUser, error) {
	return f.user, f.err
}

func serveWithAuth(
	t *testing.T,
	auth Authenticator,
	extra ...gin.HandlerFunc,
) (*gin.Engine, *bool) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RequireAuth(auth))

	called := false
	handlers := append(extra, func(c *gin.Context) {
		called = true
		c.String(http.StatusOK, "ok")
	})
	router.GET("/ping", handlers...)

	return router, &called
}

func decodeAuthError(t *testing.T, body []byte) (int, string) {
	t.Helper()

	var payload struct {
		Error struct {
			Status  int    `json:"status"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(body, &payload))

	return payload.Error.Status, payload.Error.Message
}

func TestRequireAuth_MissingCookieReturns401(t *testing.T) {
	router, called := serveWithAuth(t, &authFakeAuthenticator{})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
	status, message := decodeAuthError(t, rec.Body.Bytes())
	assert.Equal(t, http.StatusUnauthorized, status)
	assert.Equal(t, "Authentication required", message)
	assert.False(t, *called, "the handler must not run without a session cookie")
}

func TestRequireAuth_AuthenticateErrors(t *testing.T) {
	tests := []struct {
		name       string
		authErr    error
		wantStatus int
		wantMsg    string
	}{
		{
			name:       "bad token is unauthorized",
			authErr:    fmt.Errorf("authService_Authenticate: %w", errs.ErrUnauthorized),
			wantStatus: http.StatusUnauthorized,
			wantMsg:    "Authentication required",
		},
		{
			name:       "store failure is internal",
			authErr:    errors.New("redis down"),
			wantStatus: http.StatusInternalServerError,
			wantMsg:    "Internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, called := serveWithAuth(t, &authFakeAuthenticator{err: tt.authErr})

			req := httptest.NewRequest(http.MethodGet, "/ping", nil)
			req.AddCookie(&http.Cookie{Name: dto.AdminSessionCookie, Value: "token"})
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			require.Equal(t, tt.wantStatus, rec.Code, rec.Body.String())
			status, message := decodeAuthError(t, rec.Body.Bytes())
			assert.Equal(t, tt.wantStatus, status)
			assert.Equal(t, tt.wantMsg, message)
			assert.False(t, *called, "the handler must not run when authentication fails")
		})
	}
}

func TestRequireAuth_SuccessRunsHandlerAndExposesUser(t *testing.T) {
	user := &models.AdminUser{ID: uuid.New(), Login: "editor", Role: models.RoleAdmin}

	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RequireAuth(&authFakeAuthenticator{user: user}))
	router.GET("/ping", func(c *gin.Context) {
		got, ok := CurrentUser(c)
		require.True(t, ok, "RequireAuth must store the user for CurrentUser")
		assert.Equal(t, user, got)
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.AddCookie(&http.Cookie{Name: dto.AdminSessionCookie, Value: "token"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "ok", rec.Body.String())
}

func TestRequireRoles_RestrictsByRole(t *testing.T) {
	admin := &models.AdminUser{ID: uuid.New(), Login: "admin", Role: models.RoleAdmin}
	moderator := &models.AdminUser{ID: uuid.New(), Login: "moderator", Role: models.RoleModerator}

	tests := []struct {
		name       string
		user       *models.AdminUser
		wantStatus int
		wantMsg    string
		wantOK     bool
	}{
		{"admin passes", admin, http.StatusOK, "", true},
		{"moderator is forbidden", moderator, http.StatusForbidden, "Insufficient permissions", false},
		{"anonymous is unauthorized", nil, http.StatusUnauthorized, "Authentication required", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)

			router := gin.New()
			router.Use(func(c *gin.Context) {
				if tt.user != nil {
					c.Set(dto.CurrentUserKey, tt.user)
				}
				c.Next()
			})
			router.Use(RequireRoles(models.RoleAdmin))

			called := false
			router.GET("/ping", func(c *gin.Context) {
				called = true
				c.String(http.StatusOK, "ok")
			})

			req := httptest.NewRequest(http.MethodGet, "/ping", nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			require.Equal(t, tt.wantStatus, rec.Code, rec.Body.String())
			assert.Equal(t, tt.wantOK, called)

			if !tt.wantOK {
				status, message := decodeAuthError(t, rec.Body.Bytes())
				assert.Equal(t, tt.wantStatus, status)
				assert.Equal(t, tt.wantMsg, message)
			}
		})
	}
}
