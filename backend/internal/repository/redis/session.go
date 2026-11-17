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
// userSessionsKeyPrefix/Suffix name the per-user set of session token-hash hex values.
const (
	sessionKeyPrefix      = "admin:session:"
	userSessionsKeyPrefix = "admin:user:"
	userSessionsKeySuffix = ":sessions"
)

// createSessionScript stores a session, indexes it in the user's set and drops set
// members whose session already expired, so the set never outgrows the live sessions.
// Sessions share one TTL, so the newest session extends the set to the latest expiry.
// KEYS: session key, user set; ARGV: payload, ttl ms, token-hash hex, session key prefix.
const createSessionScript = `
redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[2])
for _, hash in ipairs(redis.call('SMEMBERS', KEYS[2])) do
	if redis.call('EXISTS', ARGV[4] .. hash) == 0 then
		redis.call('SREM', KEYS[2], hash)
	end
end
redis.call('SADD', KEYS[2], ARGV[3])
redis.call('PEXPIRE', KEYS[2], ARGV[2])
return 1
`

// revokeUserSessionsScript deletes every session listed in the user's set and the set
// itself in one atomic step, so a concurrent login cannot slip between read and delete.
// Session keys are derived inside the script: fine for the single-node Redis we run.
const revokeUserSessionsScript = `
local hashes = redis.call('SMEMBERS', KEYS[1])
for _, hash in ipairs(hashes) do
	redis.call('DEL', ARGV[1] .. hash)
end
redis.call('DEL', KEYS[1])
return #hashes
`

// SessionStore persists admin sessions in Redis. Each key expires with its
// session, so expired tokens disappear without a cleanup job. A per-user set
// indexes the sessions so all of them can be revoked at once.
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

	keys := []string{sessionKey(session.TokenHash), userSessionsKey(session.UserID)}
	args := []any{payload, ttl.Milliseconds(), hex.EncodeToString(session.TokenHash), sessionKeyPrefix}
	if err := s.client.Eval(ctx, createSessionScript, keys, args...).Err(); err != nil {
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

	session, err := s.GetAdminSessionByTokenHash(ctx, hash)
	if err != nil {
		if errors.Is(err, errs.ErrUnauthorized) {
			return nil
		}

		return fmt.Errorf("%s: %w", fc, err)
	}

	_, err = s.client.TxPipelined(ctx, func(pipe goredis.Pipeliner) error {
		pipe.Del(ctx, sessionKey(hash))
		pipe.SRem(ctx, userSessionsKey(session.UserID), hex.EncodeToString(hash))

		return nil
	})
	if err != nil {
		return fmt.Errorf("%s: %w", fc, err)
	}

	return nil
}

// DeleteUserSessions revokes every session of userID.
func (s *SessionStore) DeleteUserSessions(ctx context.Context, userID uuid.UUID) error {
	const fc = "sessionStore_DeleteUserSessions"

	keys := []string{userSessionsKey(userID)}
	if err := s.client.Eval(ctx, revokeUserSessionsScript, keys, sessionKeyPrefix).Err(); err != nil {
		return fmt.Errorf("%s: %w", fc, err)
	}

	return nil
}

func sessionKey(hash []byte) string {
	return sessionKeyPrefix + hex.EncodeToString(hash)
}

func userSessionsKey(userID uuid.UUID) string {
	return userSessionsKeyPrefix + userID.String() + userSessionsKeySuffix
}
