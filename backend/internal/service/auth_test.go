package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// authRepoStub stands in for the staff account persistence.
type authRepoStub struct {
	calls     int
	lastLogin string
	user      *models.AdminUser
	err       error
}

// GetAdminUserByLogin records the call and answers with the canned user or error.
func (s *authRepoStub) GetAdminUserByLogin(_ context.Context, login string) (*models.AdminUser, error) {
	s.calls++
	s.lastLogin = login

	if s.err != nil {
		return nil, s.err
	}

	return s.user, nil
}

// GetAdminUserByID answers with the canned user or error; unused by these tests.
func (s *authRepoStub) GetAdminUserByID(_ context.Context, _ uuid.UUID) (*models.AdminUser, error) {
	if s.err != nil {
		return nil, s.err
	}

	return s.user, nil
}

// authSessionStub stands in for the session persistence.
type authSessionStub struct {
	createCalls     int
	createErr       error
	deleteUserCalls int
	lastUserID      uuid.UUID
	deleteUserErr   error
}

// CreateAdminSession records the call and answers with the canned error.
func (s *authSessionStub) CreateAdminSession(_ context.Context, _ *models.AdminSession) error {
	s.createCalls++

	return s.createErr
}

// GetAdminSessionByTokenHash is unused by these tests.
func (s *authSessionStub) GetAdminSessionByTokenHash(
	_ context.Context,
	_ []byte,
) (*models.AdminSession, error) {
	return nil, fmt.Errorf("authSessionStub: %w", errs.ErrUnauthorized)
}

// DeleteAdminSession is unused by these tests.
func (s *authSessionStub) DeleteAdminSession(_ context.Context, _ []byte) error {
	return nil
}

// DeleteUserSessions records the call and answers with the canned error.
func (s *authSessionStub) DeleteUserSessions(_ context.Context, userID uuid.UUID) error {
	s.deleteUserCalls++
	s.lastUserID = userID

	return s.deleteUserErr
}

// loginLimiterStub stands in for the per-account login throttle.
type loginLimiterStub struct {
	calls   int
	lastKey string
	wait    time.Duration
	err     error
}

// AllowKey records the call and answers with the canned wait or error.
func (s *loginLimiterStub) AllowKey(_ context.Context, key string) (time.Duration, error) {
	s.calls++
	s.lastKey = key

	return s.wait, s.err
}

func newAuthTestService(
	repo AuthRepository,
	sessions SessionStore,
	limiter LoginAttemptLimiter,
) (*AuthService, *captureHandler) {
	handler := &captureHandler{}
	logger := slog.New(handler)

	return NewAuthService(repo, sessions, limiter, logger, time.Hour), handler
}

func authUserWithPassword(t *testing.T, password string) *models.AdminUser {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)

	return &models.AdminUser{
		ID:           uuid.New(),
		Login:        "admin",
		PasswordHash: string(hash),
		Role:         models.RoleAdmin,
	}
}

func TestDummyPasswordHash_IsValidBcryptHash(t *testing.T) {
	hash := dummyPasswordHash()

	cost, err := bcrypt.Cost(hash)
	require.NoError(t, err, "the dummy hash must be a real bcrypt hash")
	assert.Equal(t, bcrypt.DefaultCost, cost)

	err = bcrypt.CompareHashAndPassword(hash, []byte("x"))
	assert.ErrorIs(t, err, bcrypt.ErrMismatchedHashAndPassword,
		"a wrong password must mismatch, not fail to parse")
	assert.NotErrorIs(t, err, bcrypt.ErrHashTooShort,
		"the dummy hash must not be truncated")
}

func TestAuthService_Login_UnknownLoginReturnsUnauthorized(t *testing.T) {
	repo := &authRepoStub{
		err: fmt.Errorf("authRepoStub: %w", errs.ErrUnauthorized),
	}
	sessions := &authSessionStub{}
	svc, _ := newAuthTestService(repo, sessions, nil)

	_, err := svc.Login(context.Background(), "ghost", "some-password")

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrUnauthorized),
		"expected ErrUnauthorized, got %v", err)
}

func TestAuthService_Login_LimiterBlocksBeforeRepo(t *testing.T) {
	repo := &authRepoStub{user: authUserWithPassword(t, "correct-password")}
	sessions := &authSessionStub{}
	limiter := &loginLimiterStub{
		wait: 30 * time.Second,
		err:  fmt.Errorf("login limit: %w", errs.ErrTooManyRequests),
	}
	svc, _ := newAuthTestService(repo, sessions, limiter)

	_, err := svc.Login(context.Background(), "admin", "correct-password")

	require.Error(t, err, "a throttled login must not proceed")
	assert.True(t, errors.Is(err, errs.ErrTooManyRequests),
		"expected ErrTooManyRequests, got %v", err)

	var retryErr *errs.RetryAfterError
	require.True(t, errors.As(err, &retryErr),
		"expected *errs.RetryAfterError, got %v", err)
	assert.Equal(t, 30*time.Second, retryErr.Wait,
		"the retry wait must be the limiter TTL")
	assert.Zero(t, repo.calls, "a throttled login must not touch the repository")
	assert.Zero(t, sessions.createCalls, "a throttled login must not create a session")
}

func TestAuthService_Login_LimiterReceivesNormalizedKey(t *testing.T) {
	repo := &authRepoStub{user: authUserWithPassword(t, "correct-password")}
	sessions := &authSessionStub{}
	limiter := &loginLimiterStub{}
	svc, _ := newAuthTestService(repo, sessions, limiter)

	_, err := svc.Login(context.Background(), " Admin ", "correct-password")

	require.NoError(t, err)
	require.Equal(t, 1, limiter.calls, "the limiter must see every login attempt")
	assert.Equal(t, "admin", limiter.lastKey,
		"the limiter key must be lowercased and trimmed")
}

func TestAuthService_Login_LimiterAllowsWhenZeroWait(t *testing.T) {
	repo := &authRepoStub{user: authUserWithPassword(t, "correct-password")}
	sessions := &authSessionStub{}
	limiter := &loginLimiterStub{wait: 0, err: nil}
	svc, _ := newAuthTestService(repo, sessions, limiter)

	_, err := svc.Login(context.Background(), "admin", "correct-password")

	require.NoError(t, err, "a zero-wait limiter verdict must let the login proceed")
	assert.Equal(t, 1, repo.calls, "the repository must be consulted when the limiter allows")
}

func TestAuthService_Login_NilLimiterProceeds(t *testing.T) {
	repo := &authRepoStub{user: authUserWithPassword(t, "correct-password")}
	sessions := &authSessionStub{}
	svc, _ := newAuthTestService(repo, sessions, nil)

	result, err := svc.Login(context.Background(), "admin", "correct-password")

	require.NoError(t, err, "a nil limiter disables the per-account limit")
	require.NotNil(t, result)
	assert.Equal(t, 1, repo.calls)
	assert.Equal(t, 1, sessions.createCalls, "an allowed login must create a session")
}

func TestAuthService_Login_EmptyLoginRejectedWithoutLimiter(t *testing.T) {
	tests := []struct {
		name  string
		login string
	}{
		{"empty", ""},
		{"whitespace", "   "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &authRepoStub{user: authUserWithPassword(t, "correct-password")}
			sessions := &authSessionStub{}
			limiter := &loginLimiterStub{}
			svc, _ := newAuthTestService(repo, sessions, limiter)

			_, err := svc.Login(context.Background(), tt.login, "correct-password")

			require.Error(t, err)
			assert.True(t, errors.Is(err, errs.ErrInvalidInput),
				"expected ErrInvalidInput, got %v", err)
			assert.Zero(t, limiter.calls, "an empty login must not touch the limiter")
			assert.Zero(t, repo.calls, "an empty login must not touch the repository")
		})
	}
}

func TestAuthService_LogoutAll_RevokesUserSessions(t *testing.T) {
	userID := uuid.New()
	sessions := &authSessionStub{}
	svc, handler := newAuthTestService(&authRepoStub{}, sessions, nil)

	err := svc.LogoutAll(context.Background(), userID)

	require.NoError(t, err)
	assert.Equal(t, 1, sessions.deleteUserCalls)
	assert.Equal(t, userID, sessions.lastUserID)
	assert.Empty(t, handler.records, "a successful logout must not log")
}

func TestAuthService_LogoutAll_StoreErrorWrappedAndLogged(t *testing.T) {
	cause := errors.New("redis is down")
	sessions := &authSessionStub{deleteUserErr: cause}
	svc, handler := newAuthTestService(&authRepoStub{}, sessions, nil)

	err := svc.LogoutAll(context.Background(), uuid.New())

	require.Error(t, err)
	assert.True(t, errors.Is(err, cause),
		"the store failure must be preserved for errors.Is, got %v", err)
	assert.True(t, handler.hasFC("authService_LogoutAll"),
		"the failure must be logged with the function context")
}

func TestAuthService_LogoutAll_NilUUIDRejected(t *testing.T) {
	sessions := &authSessionStub{}
	svc, _ := newAuthTestService(&authRepoStub{}, sessions, nil)

	err := svc.LogoutAll(context.Background(), uuid.Nil)

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrInvalidInput),
		"expected ErrInvalidInput, got %v", err)
	assert.Zero(t, sessions.deleteUserCalls,
		"an invalid user id must not touch the session store")
}
