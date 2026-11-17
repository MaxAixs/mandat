package redis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// requireSessionTestClient connects to the test Redis (DB 15) or skips
// when REDIS_TEST_ADDR is not set, so unit runs never need a live Redis.
func requireSessionTestClient(t *testing.T) *goredis.Client {
	t.Helper()

	addr := os.Getenv("REDIS_TEST_ADDR")
	if addr == "" {
		t.Skip("REDIS_TEST_ADDR not set, skipping redis session integration test")
	}

	client := goredis.NewClient(&goredis.Options{Addr: addr, DB: 15})

	t.Cleanup(func() {
		_ = client.Close()
	})

	return client
}

// createSessionForUser stores a session with a random token hash for userID
// and arranges deletion of exactly the keys it created, never FLUSHDB.
func createSessionForUser(
	t *testing.T,
	ctx context.Context,
	store *SessionStore,
	client *goredis.Client,
	userID uuid.UUID,
) []byte {
	t.Helper()

	hash := make([]byte, 32)

	_, err := rand.Read(hash)
	require.NoError(t, err)

	session := &models.AdminSession{
		ID:        uuid.New(),
		UserID:    userID,
		TokenHash: hash,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}

	require.NoError(t, store.CreateAdminSession(ctx, session))

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_ = client.Del(cleanupCtx, sessionKey(hash)).Err()
		_ = store.DeleteUserSessions(cleanupCtx, userID)
	})

	return hash
}

func TestSessionStore_DeleteUserSessions_RemovesOnlyTargetUser(t *testing.T) {
	client := requireSessionTestClient(t)
	ctx := context.Background()
	store := NewSessionStore(client)

	userA := uuid.New()
	userB := uuid.New()

	hashA1 := createSessionForUser(t, ctx, store, client, userA)
	hashA2 := createSessionForUser(t, ctx, store, client, userA)
	hashB := createSessionForUser(t, ctx, store, client, userB)

	require.NoError(t, store.DeleteUserSessions(ctx, userA))

	_, err := store.GetAdminSessionByTokenHash(ctx, hashA1)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrUnauthorized),
		"the first session of user A must be gone, got %v", err)

	_, err = store.GetAdminSessionByTokenHash(ctx, hashA2)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrUnauthorized),
		"the second session of user A must be gone, got %v", err)

	loaded, err := store.GetAdminSessionByTokenHash(ctx, hashB)
	require.NoError(t, err, "the session of user B must survive")
	assert.Equal(t, userB, loaded.UserID)
}

func TestSessionStore_DeleteUserSessions_NoSessionsReturnsNil(t *testing.T) {
	client := requireSessionTestClient(t)
	ctx := context.Background()
	store := NewSessionStore(client)

	require.NoError(t, store.DeleteUserSessions(ctx, uuid.New()),
		"revoking sessions of a user with none must be a no-op")
}

func TestSessionStore_DeleteUserSessions_AfterSingleDelete(t *testing.T) {
	client := requireSessionTestClient(t)
	ctx := context.Background()
	store := NewSessionStore(client)

	userA := uuid.New()

	hashA1 := createSessionForUser(t, ctx, store, client, userA)
	hashA2 := createSessionForUser(t, ctx, store, client, userA)

	require.NoError(t, store.DeleteAdminSession(ctx, hashA1))
	require.NoError(t, store.DeleteUserSessions(ctx, userA))

	_, err := store.GetAdminSessionByTokenHash(ctx, hashA1)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrUnauthorized),
		"the singly-deleted session must stay gone, got %v", err)

	_, err = store.GetAdminSessionByTokenHash(ctx, hashA2)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrUnauthorized),
		"the remaining session of user A must be gone, got %v", err)
}

func TestSessionStore_DeleteAdminSession_RemovesHashFromUserSet(t *testing.T) {
	client := requireSessionTestClient(t)
	ctx := context.Background()
	store := NewSessionStore(client)

	userID := uuid.New()
	hash := createSessionForUser(t, ctx, store, client, userID)

	members, err := client.SMembers(ctx, userSessionsKey(userID)).Result()
	require.NoError(t, err)
	require.Contains(t, members, hex.EncodeToString(hash),
		"creating a session must index its token-hash hex in the per-user set")

	require.NoError(t, store.DeleteAdminSession(ctx, hash))

	_, err = store.GetAdminSessionByTokenHash(ctx, hash)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrUnauthorized),
		"the deleted session must be gone, got %v", err)

	members, err = client.SMembers(ctx, userSessionsKey(userID)).Result()
	require.NoError(t, err)
	assert.NotContains(t, members, hex.EncodeToString(hash),
		"logout must remove the token-hash hex from the per-user set")
}

func TestSessionStore_DeleteAdminSession_AbsentHashReturnsNil(t *testing.T) {
	client := requireSessionTestClient(t)
	ctx := context.Background()
	store := NewSessionStore(client)

	absent := make([]byte, 32)
	_, err := rand.Read(absent)
	require.NoError(t, err)

	require.NoError(t, store.DeleteAdminSession(ctx, absent),
		"deleting an absent session must be a no-op")
}

func TestSessionStore_CreateAdminSession_PrunesDeadMembers(t *testing.T) {
	client := requireSessionTestClient(t)
	ctx := context.Background()
	store := NewSessionStore(client)

	userID := uuid.New()
	hashS1 := createSessionForUser(t, ctx, store, client, userID)

	// Simulate expiry: the session key is gone while its hash lingers in the set.
	require.NoError(t, client.Del(ctx, sessionKey(hashS1)).Err())

	hashS2 := createSessionForUser(t, ctx, store, client, userID)

	members, err := client.SMembers(ctx, userSessionsKey(userID)).Result()
	require.NoError(t, err)
	assert.Contains(t, members, hex.EncodeToString(hashS2),
		"the set must index the live session")
	assert.NotContains(t, members, hex.EncodeToString(hashS1),
		"creating a session must prune hashes whose session key is already gone")
}
