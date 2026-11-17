package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A broken limiter must not lock users out: Login proceeds (fail open) and
// the swallowed infrastructure error is logged with the check function
// context instead of vanishing silently.
func TestAuthService_Login_LimiterErrorFailsOpenAndLogs(t *testing.T) {
	repo := &authRepoStub{user: authUserWithPassword(t, "correct-password")}
	sessions := &authSessionStub{}
	limiter := &loginLimiterStub{err: errors.New("limiter broken")}
	svc, handler := newAuthTestService(repo, sessions, limiter)

	result, err := svc.Login(context.Background(), "admin", "correct-password")

	require.NoError(t, err, "a limiter failure that is not a rate limit must let the login proceed")
	require.NotNil(t, result)
	assert.Equal(t, 1, repo.calls, "the repository must be consulted when the limiter fails open")
	assert.Equal(t, 1, sessions.createCalls, "a valid login must create a session when the limiter fails open")
	require.NotEmpty(t, handler.records, "a swallowed limiter error must leave a log trail")
	assert.True(t, handler.hasFC("authService_checkAttemptLimit"),
		"the log record must carry the checkAttemptLimit function context")
	assert.Equal(t, "admin", limiter.lastKey, "the limiter must still see the normalized key")
}
