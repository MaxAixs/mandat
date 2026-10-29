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
	"time"

	"github.com/google/uuid"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
	"golang.org/x/crypto/bcrypt"
)

const (
	authTokenBytes  = 32
	maxPasswordSize = 72
	dummyBcryptHash = "$2a$10$7EqJtq98hPqEX7fNZaFWoO5K4C5/RtQzj6E7n6Qm5xvB0x2b2K"
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
}

// AuthService implements staff authentication and revocable sessions.
type AuthService struct {
	repo       AuthRepository
	sessions   SessionStore
	logger     *slog.Logger
	sessionTTL time.Duration
	now        func() time.Time
}

// NewAuthService creates an AuthService over repo and sessions.
func NewAuthService(
	repo AuthRepository,
	sessions SessionStore,
	logger *slog.Logger,
	sessionTTL time.Duration,
) *AuthService {
	return &AuthService{
		repo:       repo,
		sessions:   sessions,
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

	user, err := s.repo.GetAdminUserByLogin(ctx, login)
	if err != nil {
		if errors.Is(err, errs.ErrUnauthorized) {
			if compareErr := bcrypt.CompareHashAndPassword(
				[]byte(dummyBcryptHash),
				[]byte(password),
			); compareErr != nil {
				return nil, fmt.Errorf("%s: %w", fc, errs.ErrUnauthorized)
			}

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
