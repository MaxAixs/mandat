package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
	"github.com/maxaixs/mandat/backend/internal/service"
	httptransport "github.com/maxaixs/mandat/backend/internal/transport/http"
	"github.com/maxaixs/mandat/backend/internal/transport/http/dto"
	"github.com/maxaixs/mandat/backend/internal/transport/http/handler"
	"github.com/maxaixs/mandat/backend/internal/transport/http/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubAuthService implements handler.AuthService with canned answers.
type stubAuthService struct {
	users           map[string]*models.AdminUser
	authenticateErr error
	loginResult     *service.AuthResult
	loginErr        error
	logoutAllErr    error
	logoutAllCalls  int
	logoutAllUserID uuid.UUID
}

func (s *stubAuthService) Login(
	_ context.Context,
	_ string,
	_ string,
) (*service.AuthResult, error) {
	return s.loginResult, s.loginErr
}

func (s *stubAuthService) Authenticate(
	_ context.Context,
	rawToken string,
) (*models.AdminUser, error) {
	if s.authenticateErr != nil {
		return nil, s.authenticateErr
	}

	user, ok := s.users[rawToken]
	if !ok {
		return nil, errs.ErrUnauthorized
	}

	return user, nil
}

func (s *stubAuthService) Logout(_ context.Context, _ string) error {
	return nil
}

func (s *stubAuthService) LogoutAll(_ context.Context, userID uuid.UUID) error {
	s.logoutAllCalls++
	s.logoutAllUserID = userID

	return s.logoutAllErr
}

func authPassthrough(c *gin.Context) {
	c.Next()
}

func setupAuthRouter(t *testing.T, stub *stubAuthService) http.Handler {
	t.Helper()

	httpSrv, err := httptransport.New(
		httptransport.HTTPConfig{},
		nil,
		handler.NewAuthHandler(stub, false, middleware.RequireAuth(stub), authPassthrough),
	)
	require.NoError(t, err)

	return httpSrv.Handler()
}

func doAuthRequest(
	t *testing.T,
	router http.Handler,
	method string,
	path string,
	body string,
	cookies ...*http.Cookie,
) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}

	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	return rec
}

func authSessionCookie(value string) *http.Cookie {
	return &http.Cookie{Name: dto.AdminSessionCookie, Value: value}
}

func authMePayload(t *testing.T, body []byte) map[string]any {
	t.Helper()

	var payload map[string]any
	require.NoError(t, json.Unmarshal(body, &payload))

	return payload
}

func clearedSessionCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()

	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == dto.AdminSessionCookie {
			return cookie
		}
	}

	return nil
}

func TestAuthLogoutAll_RevokesSessionsAndClearsCookie(t *testing.T) {
	user := &models.AdminUser{ID: uuid.New(), Login: "editor", Role: models.RoleAdmin}
	stub := &stubAuthService{users: map[string]*models.AdminUser{"tok-1": user}}
	router := setupAuthRouter(t, stub)

	rec := doAuthRequest(t, router, http.MethodPost, "/api/auth/logout-all", "",
		authSessionCookie("tok-1"))

	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	assert.Equal(t, 1, stub.logoutAllCalls, "logout-all must revoke exactly once")
	assert.Equal(t, user.ID, stub.logoutAllUserID)

	cleared := clearedSessionCookie(t, rec)
	require.NotNil(t, cleared, "logout-all must clear the session cookie")
	assert.Empty(t, cleared.Value)
	assert.LessOrEqual(t, cleared.MaxAge, 0, "clearing cookie must expire immediately")
}

func TestAuthLogoutAll_UnauthenticatedReturns401(t *testing.T) {
	stub := &stubAuthService{users: map[string]*models.AdminUser{}}
	router := setupAuthRouter(t, stub)

	rec := doAuthRequest(t, router, http.MethodPost, "/api/auth/logout-all", "")

	require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
	assert.Equal(t, 0, stub.logoutAllCalls, "LogoutAll must not run without a session")

	body := decodeError(t, rec.Body.Bytes())
	assert.Equal(t, http.StatusUnauthorized, body.Error.Status)
	assert.Equal(t, "Authentication required", body.Error.Message)
}

func TestAuthLogoutAll_ServiceErrorReturns500WithoutClearingCookie(t *testing.T) {
	user := &models.AdminUser{ID: uuid.New(), Login: "editor", Role: models.RoleAdmin}
	stub := &stubAuthService{
		users:        map[string]*models.AdminUser{"tok-1": user},
		logoutAllErr: errors.New("redis down"),
	}
	router := setupAuthRouter(t, stub)

	rec := doAuthRequest(t, router, http.MethodPost, "/api/auth/logout-all", "",
		authSessionCookie("tok-1"))

	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
	assert.Equal(t, 1, stub.logoutAllCalls)

	body := decodeError(t, rec.Body.Bytes())
	assert.Equal(t, http.StatusInternalServerError, body.Error.Status)
	assert.Equal(t, "Internal server error", body.Error.Message)

	cleared := clearedSessionCookie(t, rec)
	if cleared != nil {
		assert.NotEqual(t, "", cleared.Value,
			"a failed logout-all must not clear the session cookie")
		assert.Greater(t, cleared.MaxAge, 0,
			"a failed logout-all must not expire the session cookie")
	}
}

func TestAuthLogin_RateLimitedReturns429WithRetryAfter(t *testing.T) {
	stub := &stubAuthService{loginErr: errs.NewRetryAfterError(30 * time.Second)}
	router := setupAuthRouter(t, stub)

	rec := doAuthRequest(t, router, http.MethodPost, "/api/auth/login",
		`{"login":"editor","password":"secret"}`)

	require.Equal(t, http.StatusTooManyRequests, rec.Code, rec.Body.String())
	assert.Equal(t, "30", rec.Header().Get("Retry-After"))

	body := decodeError(t, rec.Body.Bytes())
	assert.Equal(t, http.StatusTooManyRequests, body.Error.Status)
	assert.Equal(t, "Too many requests", body.Error.Message)
}

func TestAuthMe_ReturnsCurrentUser(t *testing.T) {
	user := &models.AdminUser{ID: uuid.New(), Login: "editor", Role: models.RoleAdmin}
	stub := &stubAuthService{users: map[string]*models.AdminUser{"tok-1": user}}
	router := setupAuthRouter(t, stub)

	rec := doAuthRequest(t, router, http.MethodGet, "/api/auth/me", "",
		authSessionCookie("tok-1"))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	payload := authMePayload(t, rec.Body.Bytes())
	assert.Equal(t, user.ID.String(), payload["id"])
	assert.Equal(t, "editor", payload["login"])
	assert.Equal(t, string(models.RoleAdmin), payload["role"])
}
