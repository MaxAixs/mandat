package service

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// coverValidationService builds a cover-enabled service wired to handler so
// validation tests can assert that no error-level record is emitted.
func coverValidationService(
	repo *testsupport.StubRepo,
	storage *fakeImageStorage,
	handler *captureHandler,
) *NewsService {
	return newCoverLoggingService(repo, storage, handler)
}

func TestService_Create_CoverBytesMismatchFailsBeforeWrite(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		body     []byte
	}{
		{"png bytes named jpg", "x.jpg", fetchTestPNG},
		{"jpeg bytes named png", "x.png", fetchTestJPEG},
		{"plain text named png", "x.png", []byte("hello")},
		{"plain text named jpg", "x.jpg", []byte("hello")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &fakeImageStorage{uploadURL: coverTestPublicBaseURL + "/covers/mismatch.jpg"}
			repo := testsupport.NewStubRepo()
			handler := &captureHandler{}
			svc := coverValidationService(repo, storage, handler)

			input := validInput()
			input.Cover = &CoverUpload{
				Body:        bytes.NewReader(tt.body),
				Filename:    tt.filename,
				ContentType: "image/jpeg",
				Size:        int64(len(tt.body)),
			}

			created, err := svc.Create(context.Background(), input)

			require.Error(t, err)
			assert.Nil(t, created)
			var validationErr *errs.ValidationError
			require.True(t, errors.As(err, &validationErr),
				"a cover whose bytes do not match its extension must be a ValidationError, got %v", err)
			assert.Equal(t, "cover", validationErr.Field)
			assert.Zero(t, repo.CreateCalls, "a mismatched cover must be rejected before any repository write")
			assert.Empty(t, storage.uploadKeys, "a mismatched cover must never be uploaded")
			assert.Empty(t, repo.Items)
			assert.Zero(t, countErrorRecords(handler),
				"a cover validation failure must not emit an error-level log record")
		})
	}
}

func TestService_Update_CoverBytesMismatchFailsBeforeWrite(t *testing.T) {
	storage := &fakeImageStorage{uploadURL: coverTestPublicBaseURL + "/covers/mismatch.jpg"}
	repo := &updateCountingRepo{StubRepo: testsupport.NewStubRepo()}
	handler := &captureHandler{}
	svc := coverValidationService(repo.StubRepo, storage, handler)
	ctx := context.Background()

	existing, err := svc.Create(ctx, validInput())
	require.NoError(t, err)
	require.Zero(t, repo.updateCalls)

	input := validInput()
	input.Cover = &CoverUpload{
		Body:        strings.NewReader(string(fetchTestPNG)),
		Filename:    "x.jpg",
		ContentType: "image/jpeg",
		Size:        int64(len(fetchTestPNG)),
	}

	updated, err := svc.Update(ctx, existing.Slug, input)

	require.Error(t, err)
	assert.Nil(t, updated)
	var validationErr *errs.ValidationError
	require.True(t, errors.As(err, &validationErr),
		"a cover whose bytes do not match its extension must be a ValidationError, got %v", err)
	assert.Equal(t, "cover", validationErr.Field)
	assert.Zero(t, repo.updateCalls, "a mismatched cover must be rejected before any repository write")
	assert.Empty(t, storage.uploadKeys, "a mismatched cover must never be uploaded")
	assert.Zero(t, countErrorRecords(handler),
		"a cover validation failure must not emit an error-level log record")
}

func TestService_Create_OversizedCoverFailsBeforeWrite(t *testing.T) {
	const oversized = (10 << 20) + 1

	storage := &fakeImageStorage{uploadURL: coverTestPublicBaseURL + "/covers/huge.jpg"}
	repo := testsupport.NewStubRepo()
	handler := &captureHandler{}
	svc := coverValidationService(repo, storage, handler)

	body := make([]byte, oversized)
	copy(body, fetchTestJPEG)
	input := validInput()
	input.Cover = &CoverUpload{
		Body:        bytes.NewReader(body),
		Filename:    "huge.jpg",
		ContentType: "image/jpeg",
		Size:        oversized,
	}

	created, err := svc.Create(context.Background(), input)

	require.Error(t, err)
	assert.Nil(t, created)
	assert.True(t, errors.Is(err, errs.ErrPayloadTooLarge),
		"a cover larger than 10 MB must surface ErrPayloadTooLarge, got %v", err)
	assert.Zero(t, repo.CreateCalls, "an oversized cover must be rejected before any repository write")
	assert.Empty(t, storage.uploadKeys, "an oversized cover must never be uploaded")
	assert.Empty(t, repo.Items)
	assert.Zero(t, countErrorRecords(handler),
		"an oversized cover must not emit an error-level log record")
}

func TestService_Update_OversizedCoverFailsBeforeWrite(t *testing.T) {
	const oversized = (10 << 20) + 1

	storage := &fakeImageStorage{}
	repo := &updateCountingRepo{StubRepo: testsupport.NewStubRepo()}
	handler := &captureHandler{}
	svc := coverValidationService(repo.StubRepo, storage, handler)
	ctx := context.Background()

	existing, err := svc.Create(ctx, validInput())
	require.NoError(t, err)
	require.Zero(t, repo.updateCalls)

	body := make([]byte, oversized)
	copy(body, fetchTestPNG)
	input := validInput()
	input.Cover = &CoverUpload{
		Body:        bytes.NewReader(body),
		Filename:    "huge.png",
		ContentType: "image/png",
		Size:        oversized,
	}

	updated, err := svc.Update(ctx, existing.Slug, input)

	require.Error(t, err)
	assert.Nil(t, updated)
	assert.True(t, errors.Is(err, errs.ErrPayloadTooLarge),
		"a cover larger than 10 MB must surface ErrPayloadTooLarge, got %v", err)
	assert.Zero(t, repo.updateCalls, "an oversized cover must be rejected before any repository write")
	assert.Empty(t, storage.uploadKeys, "an oversized cover must never be uploaded")
	assert.Zero(t, countErrorRecords(handler),
		"an oversized cover must not emit an error-level log record")
}

func TestService_Create_UploadFailureRemovesRowAndFails(t *testing.T) {
	storage := &fakeImageStorage{uploadErr: errors.New("r2 down")}
	repo := testsupport.NewStubRepo()
	handler := &captureHandler{}
	svc := coverValidationService(repo, storage, handler)

	input := validInput()
	input.Cover = &CoverUpload{
		Body:        bytes.NewReader(fetchTestJPEG),
		Filename:    "cover.jpg",
		ContentType: "image/jpeg",
		Size:        int64(len(fetchTestJPEG)),
	}

	created, err := svc.Create(context.Background(), input)

	require.Error(t, err, "an upload failure after the row was inserted must fail the create")
	assert.Nil(t, created)
	assert.True(t, errors.Is(err, errs.ErrServiceUnavailable),
		"an upload failure must surface ErrServiceUnavailable, got %v", err)
	assert.Empty(t, repo.Items, "the created row must be removed when the upload fails")
}

func TestService_Create_CoverURLSaveFailureRemovesRowAndObject(t *testing.T) {
	storage := &fakeImageStorage{uploadURL: coverTestPublicBaseURL + "/covers/saved.jpg"}
	inner := testsupport.NewStubRepo()
	handler := &captureHandler{}
	svc := coverValidationService(inner, storage, handler)

	input := validInput()
	input.Cover = &CoverUpload{
		Body:        bytes.NewReader(fetchTestJPEG),
		Filename:    "cover.jpg",
		ContentType: "image/jpeg",
		Size:        int64(len(fetchTestJPEG)),
	}
	inner.UpdateErr = errors.New("database is down")

	created, err := svc.Create(context.Background(), input)

	require.Error(t, err, "a failed cover URL save must fail the create")
	assert.Nil(t, created)
	require.Len(t, storage.uploadKeys, 1, "the cover upload must be attempted once")
	assert.Equal(t, storage.uploadKeys, storage.deleteKeys,
		"the uploaded object must be deleted when saving its URL fails")
	assert.Empty(t, inner.Items, "the created row must be removed when saving its URL fails")
}
