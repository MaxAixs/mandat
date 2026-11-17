package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
	"golang.org/x/crypto/bcrypt"
)

const (
	authTokenBytes  = 32
	maxPasswordSize = 72
	// dummyPassword feeds the timing-equalizing hash; it never matches a real account.
	dummyPassword = "mandat-dummy-password"
)

// AuthResult is a successful login and its browser token.
type AuthResult struct {
	User      *models.AdminUser
	Token     string
	ExpiresAt time.Time
}

// AuthRepository is the staff account persistence contract used by AuthService.
type AuthRepository interface {
	GetAdminUserByLogin(ctx context.Context, login string) (*models.AdminUser, error)
	GetAdminUserByID(ctx context.Context, id uuid.UUID) (*models.AdminUser, error)
}

// SessionStore is the session persistence contract used by AuthService.
type SessionStore interface {
	CreateAdminSession(ctx context.Context, session *models.AdminSession) error
	GetAdminSessionByTokenHash(ctx context.Context, hash []byte) (*models.AdminSession, error)
	DeleteAdminSession(ctx context.Context, hash []byte) error
	DeleteUserSessions(ctx context.Context, userID uuid.UUID) error
}

// LoginAttemptLimiter throttles login attempts per account.
type LoginAttemptLimiter interface {
	Allow(ctx context.Context, key string) (time.Duration, error)
}

// dummyPasswordHash is compared against when the login is unknown,
// so unknown and known accounts cost the same bcrypt work.
var dummyPasswordHash = sync.OnceValue(func() []byte {
	hash, err := bcrypt.GenerateFromPassword([]byte(dummyPassword), bcrypt.DefaultCost)
	if err != nil {
		panic(fmt.Sprintf("generate dummy bcrypt hash: %v", err))
	}

	return hash
})

// AuthService implements staff authentication and revocable sessions.
type AuthService struct {
	repo       AuthRepository
	sessions   SessionStore
	limiter    LoginAttemptLimiter
	logger     *slog.Logger
	sessionTTL time.Duration
	now        func() time.Time
}

// NewAuthService creates an AuthService over repo and sessions.
// A nil limiter disables the per-account login limit.
func NewAuthService(
	repo AuthRepository,
	sessions SessionStore,
	limiter LoginAttemptLimiter,
	logger *slog.Logger,
	sessionTTL time.Duration,
) *AuthService {
	return &AuthService{
		repo:       repo,
		sessions:   sessions,
		limiter:    limiter,
		logger:     logger,
		sessionTTL: sessionTTL,
		now:        time.Now,
	}
}

// Login verifies staff credentials and creates a new opaque session token.
func (s *AuthService) Login(ctx context.Context, login string, password string) (*AuthResult, error) {
	const fc = "authService_Login"

	login = strings.TrimSpace(login)
	if login == "" || password == "" || len(password) > maxPasswordSize {
		return nil, fmt.Errorf("%s: %w", fc, errs.ErrInvalidInput)
	}

	if err := s.checkAttemptLimit(ctx, login); err != nil {
		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	user, err := s.repo.GetAdminUserByLogin(ctx, login)
	if err != nil {
		if errors.Is(err, errs.ErrUnauthorized) {
			// Unknown logins pay the same bcrypt cost, so timing does not reveal accounts.
			_ = bcrypt.CompareHashAndPassword(dummyPasswordHash(), []byte(password))

			return nil, fmt.Errorf("%s: %w", fc, errs.ErrUnauthorized)
		}

		s.logger.ErrorContext(ctx, "failed to load staff account", "fc", fc, "error", err)

		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, fmt.Errorf("%s: %w", fc, errs.ErrUnauthorized)
	}

	token, tokenHash, err := generateAuthToken()
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to generate session token", "fc", fc, "error", err)

		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	now := s.now().UTC()
	session := &models.AdminSession{
		ID:        uuid.New(),
		UserID:    user.ID,
		TokenHash: tokenHash,
		CreatedAt: now,
		ExpiresAt: now.Add(s.sessionTTL),
	}
	if err := s.sessions.CreateAdminSession(ctx, session); err != nil {
		s.logger.ErrorContext(ctx, "failed to create auth session", "fc", fc, "error", err)

		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	return &AuthResult{User: user, Token: token, ExpiresAt: session.ExpiresAt}, nil
}

// checkAttemptLimit counts a login attempt against the account and rejects it
// with the wait time once the account is over its limit.
func (s *AuthService) checkAttemptLimit(ctx context.Context, login string) error {
	const fc = "authService_checkAttemptLimit"

	if s.limiter == nil {
		return nil
	}

	wait, err := s.limiter.Allow(ctx, strings.ToLower(login))
	if err == nil {
		return nil
	}
	if errors.Is(err, errs.ErrTooManyRequests) {
		return fmt.Errorf("%s: %w", fc, errs.NewRetryAfterError(wait))
	}

	// Fail open like the global limit: a broken limiter must not lock staff out.
	s.logger.ErrorContext(ctx, "login attempt limit check failed, attempt allowed", "fc", fc, "error", err)

	return nil
}

// Authenticate resolves a browser token to a staff user.
func (s *AuthService) Authenticate(ctx context.Context, rawToken string) (*models.AdminUser, error) {
	const fc = "authService_Authenticate"

	tokenHash, err := authTokenHash(rawToken)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fc, errs.ErrUnauthorized)
	}

	session, err := s.sessions.GetAdminSessionByTokenHash(ctx, tokenHash)
	if err != nil {
		if !errors.Is(err, errs.ErrUnauthorized) {
			s.logger.ErrorContext(ctx, "failed to load auth session", "fc", fc, "error", err)
		}

		return nil, fmt.Errorf("%s: %w", fc, err)
	}
	if !session.ExpiresAt.After(s.now().UTC()) {
		return nil, fmt.Errorf("%s: %w", fc, errs.ErrUnauthorized)
	}

	// The account is always read fresh: role changes apply without re-login.
	user, err := s.repo.GetAdminUserByID(ctx, session.UserID)
	if err != nil {
		if !errors.Is(err, errs.ErrUnauthorized) {
			s.logger.ErrorContext(ctx, "failed to load staff account", "fc", fc, "error", err)
		}

		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	return user, nil
}

// Logout revokes a browser session token.
func (s *AuthService) Logout(ctx context.Context, rawToken string) error {
	const fc = "authService_Logout"

	tokenHash, err := authTokenHash(rawToken)
	if err != nil {
		return fmt.Errorf("%s: %w", fc, errs.ErrUnauthorized)
	}
	if err := s.sessions.DeleteAdminSession(ctx, tokenHash); err != nil {
		s.logger.ErrorContext(ctx, "failed to revoke auth session", "fc", fc, "error", err)

		return fmt.Errorf("%s: %w", fc, err)
	}

	return nil
}

// LogoutAll revokes every session of the user.
func (s *AuthService) LogoutAll(ctx context.Context, userID uuid.UUID) error {
	const fc = "authService_LogoutAll"

	if userID == uuid.Nil {
		return fmt.Errorf("%s: %w", fc, errs.ErrInvalidInput)
	}
	if err := s.sessions.DeleteUserSessions(ctx, userID); err != nil {
		s.logger.ErrorContext(ctx, "failed to revoke user sessions", "fc", fc, "user_id", userID, "error", err)

		return fmt.Errorf("%s: %w", fc, err)
	}

	return nil
}

func generateAuthToken() (string, []byte, error) {
	const fc = "authService_generateAuthToken"

	bytes := make([]byte, authTokenBytes)
	if _, err := rand.Read(bytes); err != nil {
		return "", nil, fmt.Errorf("%s: %w", fc, err)
	}

	token := base64.RawURLEncoding.EncodeToString(bytes)
	hash := sha256.Sum256(bytes)

	return token, hash[:], nil
}

func authTokenHash(rawToken string) ([]byte, error) {
	if rawToken == "" {
		return nil, fmt.Errorf("empty token")
	}

	decoded, err := base64.RawURLEncoding.DecodeString(rawToken)
	if err != nil {
		return nil, fmt.Errorf("decode token: %w", err)
	}
	if len(decoded) != authTokenBytes {
		return nil, fmt.Errorf("invalid token length")
	}

	hash := sha256.Sum256(decoded)
	return hash[:], nil
}
