package service

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
	"github.com/maxaixs/mandat/backend/internal/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newPublicTestService() (*NewsService, *testsupport.StubRepo, *captureHandler) {
	repo := testsupport.NewStubRepo()
	handler := &captureHandler{}

	return NewNewsService(repo, slog.New(handler)), repo, handler
}

// createPublished stores a published article and returns it.
func createPublished(t *testing.T, svc *NewsService, title string) *models.News {
	t.Helper()

	input := validInput()
	input.Title = title
	input.Status = string(models.StatusPublished)

	created, err := svc.Create(context.Background(), input)
	require.NoError(t, err)

	return created
}

func TestListPublished_ReturnsOnlyPublishedArticles(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()

	draft, err := svc.Create(ctx, validInput())
	require.NoError(t, err)
	require.Equal(t, models.StatusDraft, draft.Status)

	published := createPublished(t, svc, "Перша публікація")

	result, err := svc.ListPublished(ctx, 1, 10)
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	assert.Equal(t, published.Slug, result.Items[0].Slug)
	assert.NotEqual(t, draft.Slug, result.Items[0].Slug, "drafts must never reach the public list")
	assert.Equal(t, 1, result.Page)
	assert.Equal(t, 10, result.Limit)
}

func TestListPublished_Validation(t *testing.T) {
	tests := []struct {
		name  string
		page  int
		limit int
	}{
		{"zero page", 0, 10},
		{"negative page", -1, 10},
		{"zero limit", 1, 0},
		{"negative limit", 1, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo := newTestService()

			_, err := svc.ListPublished(context.Background(), tt.page, tt.limit)

			require.Error(t, err)
			assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
			assert.Equal(t, models.ListFilter{}, repo.LastPublishedFilter,
				"invalid pagination must not reach the repository")
		})
	}
}

func TestListPublished_ClampsLimitToMax(t *testing.T) {
	svc, repo := newTestService()

	result, err := svc.ListPublished(context.Background(), 3, 100000)
	require.NoError(t, err)

	assert.Equal(t, MaxPageLimit, result.Limit)
	assert.Equal(t, 3, result.Page)
	assert.Equal(t, MaxPageLimit, repo.LastPublishedFilter.Limit)
	assert.Equal(t, 3, repo.LastPublishedFilter.Page)
	assert.Nil(t, repo.LastPublishedFilter.Status, "the public list must not let callers choose a status")
}

func TestListPublished_RepoFailurePropagatesAndLogs(t *testing.T) {
	cause := errors.New("db is down")
	svc, repo, handler := newPublicTestService()
	repo.ListPublishedErr = cause

	_, err := svc.ListPublished(context.Background(), 1, 10)

	require.Error(t, err)
	assert.True(t, errors.Is(err, cause), "the repository cause must be preserved, got %v", err)
	assert.True(t, handler.hasFC("newsService_ListPublished"),
		"expected a log record carrying the fc identifier")
}
