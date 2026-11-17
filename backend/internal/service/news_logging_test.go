package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"

	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countErrorRecords counts the error-level records a service call emitted.
func countErrorRecords(h *captureHandler) int {
	count := 0
	for _, record := range h.records {
		if record.Level >= slog.LevelError {
			count++
		}
	}

	return count
}

// loggingService builds a NewsService wired to handler for log assertions.
func loggingService(repo *testsupport.StubRepo, handler *captureHandler) *NewsService {
	return NewNewsService(repo, slog.New(handler), nil, "", nil)
}

func TestGetBySlug_MissingEmitsNoErrorLog(t *testing.T) {
	repo := testsupport.NewStubRepo()
	handler := &captureHandler{}
	svc := loggingService(repo, handler)

	_, err := svc.GetBySlug(context.Background(), "missing-article")

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrNewsNotFound), "expected ErrNewsNotFound, got %v", err)
	assert.Zero(t, countErrorRecords(handler),
		"a missing article is an expected outcome and must not be logged as an error")
}

func TestGetBySlug_InfraFailureStillLogs(t *testing.T) {
	cause := errors.New("db is down")
	repo := testsupport.NewStubRepo()
	repo.GetErr = cause
	handler := &captureHandler{}
	svc := loggingService(repo, handler)

	_, err := svc.GetBySlug(context.Background(), "any-article")

	require.Error(t, err)
	assert.True(t, errors.Is(err, cause), "the repository cause must be preserved, got %v", err)
	assert.True(t, handler.hasFC("newsService_GetBySlug"),
		"an infrastructure failure must still be logged with its function context")
}

func TestGetPublishedBySlug_MissingEmitsNoErrorLog(t *testing.T) {
	repo := testsupport.NewStubRepo()
	handler := &captureHandler{}
	svc := loggingService(repo, handler)

	_, err := svc.GetPublishedBySlug(context.Background(), "missing-article")

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrNewsNotFound), "expected ErrNewsNotFound, got %v", err)
	assert.Zero(t, countErrorRecords(handler),
		"a missing published article is an expected outcome and must not be logged as an error")
}

func TestGetPublishedBySlug_DraftEmitsNoErrorLog(t *testing.T) {
	repo := testsupport.NewStubRepo()
	handler := &captureHandler{}
	svc := loggingService(repo, handler)

	draft, err := svc.Create(context.Background(), validInput())
	require.NoError(t, err)

	_, err = svc.GetPublishedBySlug(context.Background(), draft.Slug)

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrNewsNotFound), "expected ErrNewsNotFound, got %v", err)
	assert.Zero(t, countErrorRecords(handler),
		"a draft behind the public endpoint is an expected outcome and must not be logged as an error")
}

func TestUpdate_MissingEmitsNoErrorLog(t *testing.T) {
	repo := testsupport.NewStubRepo()
	handler := &captureHandler{}
	svc := loggingService(repo, handler)

	_, err := svc.Update(context.Background(), "missing-article", validInput())

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrNewsNotFound), "expected ErrNewsNotFound, got %v", err)
	assert.Zero(t, countErrorRecords(handler),
		"updating a missing article is an expected outcome and must not be logged as an error")
}

func TestDelete_MissingEmitsNoErrorLog(t *testing.T) {
	repo := testsupport.NewStubRepo()
	handler := &captureHandler{}
	svc := loggingService(repo, handler)

	err := svc.Delete(context.Background(), "missing-article")

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrNewsNotFound), "expected ErrNewsNotFound, got %v", err)
	assert.Zero(t, countErrorRecords(handler),
		"deleting a missing article is an expected outcome and must not be logged as an error")
}

func TestCreate_ValidationFailureEmitsNoErrorLog(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(input *NewsInput)
	}{
		{"empty title", func(in *NewsInput) { in.Title = "" }},
		{"unknown category", func(in *NewsInput) { in.Category = "Фантастика" }},
		{"published without text", func(in *NewsInput) {
			in.Status = "published"
			in.Text = ""
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := testsupport.NewStubRepo()
			handler := &captureHandler{}
			svc := loggingService(repo, handler)
			input := validInput()
			tt.mutate(&input)

			_, err := svc.Create(context.Background(), input)

			require.Error(t, err)
			assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
			assert.Zero(t, countErrorRecords(handler),
				"a validation failure is an expected outcome and must not be logged as an error")
		})
	}
}

func TestUpdate_ValidationFailureEmitsNoErrorLog(t *testing.T) {
	repo := testsupport.NewStubRepo()
	handler := &captureHandler{}
	svc := loggingService(repo, handler)
	ctx := context.Background()

	created, err := svc.Create(ctx, validInput())
	require.NoError(t, err)
	require.Zero(t, countErrorRecords(handler))

	update := validInput()
	update.Title = ""

	_, err = svc.Update(ctx, created.Slug, update)

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
	assert.Zero(t, countErrorRecords(handler),
		"a validation failure is an expected outcome and must not be logged as an error")
}

func TestCreate_SlugConflictEmitsNoErrorLog(t *testing.T) {
	repo := testsupport.NewStubRepo()
	handler := &captureHandler{}
	svc := loggingService(repo, handler)

	repo.Conflicts["uriad-skhvalyv-novyi-biudzhet"] = true
	for attempt := 2; attempt <= maxSlugAttempts; attempt++ {
		repo.Conflicts[fmt.Sprintf("uriad-skhvalyv-novyi-biudzhet-%d", attempt)] = true
	}

	_, err := svc.Create(context.Background(), validInput())

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrConflict), "expected ErrConflict, got %v", err)
	assert.Zero(t, countErrorRecords(handler),
		"an exhausted slug retry is an expected outcome and must not be logged as an error")
}
