// Package redis contains the Redis-backed repository implementations.
package redis

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
	goredis "github.com/redis/go-redis/v9"
)

// sessionKeyPrefix namespaces admin sessions inside the shared Redis database.
const sessionKeyPrefix = "admin:session:"

// SessionStore persists admin sessions in Redis. Each key expires with its
// session, so expired tokens disappear without a cleanup job.
type SessionStore struct {
	client *goredis.Client
}

// NewSessionStore creates a SessionStore backed by client.
func NewSessionStore(client *goredis.Client) *SessionStore {
	return &SessionStore{client: client}
}

// sessionRecord is the serialized session payload; the token hash lives in the key.
type sessionRecord struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// CreateAdminSession stores a session until its expiry time.
func (s *SessionStore) CreateAdminSession(ctx context.Context, session *models.AdminSession) error {
	const fc = "sessionStore_CreateAdminSession"

	ttl := time.Until(session.ExpiresAt)
	if ttl <= 0 {
		return fmt.Errorf("%s: %w", fc, errs.ErrInvalidInput)
	}

	payload, err := json.Marshal(sessionRecord{
		ID:        session.ID,
		UserID:    session.UserID,
		CreatedAt: session.CreatedAt,
		ExpiresAt: session.ExpiresAt,
	})
	if err != nil {
		return fmt.Errorf("%s: encode session: %w", fc, err)
	}

	if err := s.client.Set(ctx, sessionKey(session.TokenHash), payload, ttl).Err(); err != nil {
		return fmt.Errorf("%s: %w", fc, err)
	}

	return nil
}

// GetAdminSessionByTokenHash returns a session or errs.ErrUnauthorized when absent.
func (s *SessionStore) GetAdminSessionByTokenHash(ctx context.Context, hash []byte) (*models.AdminSession, error) {
	const fc = "sessionStore_GetAdminSessionByTokenHash"

	payload, err := s.client.Get(ctx, sessionKey(hash)).Bytes()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, fmt.Errorf("%s: %w", fc, errs.ErrUnauthorized)
		}

		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	var record sessionRecord
	if err := json.Unmarshal(payload, &record); err != nil {
		return nil, fmt.Errorf("%s: decode session: %w", fc, err)
	}

	return &models.AdminSession{
		ID:        record.ID,
		UserID:    record.UserID,
		TokenHash: hash,
		CreatedAt: record.CreatedAt,
		ExpiresAt: record.ExpiresAt,
	}, nil
}

// DeleteAdminSession revokes a session; deleting an absent key is not an error.
func (s *SessionStore) DeleteAdminSession(ctx context.Context, hash []byte) error {
	const fc = "sessionStore_DeleteAdminSession"

	if err := s.client.Del(ctx, sessionKey(hash)).Err(); err != nil {
		return fmt.Errorf("%s: %w", fc, err)
	}

	return nil
}

func sessionKey(hash []byte) string {
	return sessionKeyPrefix + hex.EncodeToString(hash)
}
