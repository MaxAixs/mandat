package service

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Without cover storage a valid upload has nowhere to go: Create must fail
// with ErrServiceUnavailable before any row is written.
func TestService_Create_NilStorageWithCoverFailsUnavailable(t *testing.T) {
	repo := testsupport.NewStubRepo()
	svc := NewNewsService(repo, slog.New(&captureHandler{}), nil, coverTestPublicBaseURL, nil)

	input := validInput()
	input.Cover = &CoverUpload{
		Body:        bytes.NewReader(fetchTestJPEG),
		Filename:    "x.jpg",
		ContentType: "image/jpeg",
		Size:        int64(len(fetchTestJPEG)),
	}

	created, err := svc.Create(context.Background(), input)

	require.Error(t, err)
	assert.Nil(t, created)
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable),
		"a cover with nil storage must surface ErrServiceUnavailable, got %v", err)
	assert.Zero(t, repo.CreateCalls, "no row may be written when cover storage is not configured")
	assert.Empty(t, repo.Items)
}

// Without cover storage a rotation has nowhere to go: Update must fail with
// ErrServiceUnavailable before the repository is written.
func TestService_Update_NilStorageWithCoverFailsUnavailable(t *testing.T) {
	repo := &updateCountingRepo{StubRepo: testsupport.NewStubRepo()}
	plain := NewNewsService(repo.StubRepo, slog.New(&captureHandler{}), nil, "", nil)

	seed, err := plain.Create(context.Background(), validInput())
	require.NoError(t, err)
	require.Zero(t, repo.updateCalls)

	svc := NewNewsService(repo.StubRepo, slog.New(&captureHandler{}), nil, coverTestPublicBaseURL, nil)

	input := validInput()
	input.Cover = &CoverUpload{
		Body:        bytes.NewReader(fetchTestJPEG),
		Filename:    "x.jpg",
		ContentType: "image/jpeg",
		Size:        int64(len(fetchTestJPEG)),
	}

	updated, err := svc.Update(context.Background(), seed.Slug, input)

	require.Error(t, err)
	assert.Nil(t, updated)
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable),
		"a cover with nil storage must surface ErrServiceUnavailable, got %v", err)
	assert.Zero(t, repo.updateCalls, "no row may be updated when cover storage is not configured")
}
