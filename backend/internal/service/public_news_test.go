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

// publishedDetailService is the contract the public article page needs.
// Asserting it keeps these tests compiling until the method exists.
type publishedDetailService interface {
	GetPublishedBySlug(ctx context.Context, slug string) (*models.News, error)
}

// requirePublishedDetail returns svc as a publishedDetailService.
func requirePublishedDetail(t *testing.T, svc *NewsService) publishedDetailService {
	t.Helper()

	detail, ok := any(svc).(publishedDetailService)
	require.True(t, ok, "NewsService must implement GetPublishedBySlug(ctx, slug)")

	return detail
}

func TestGetPublishedBySlug_ReturnsPublishedArticle(t *testing.T) {
	svc, _, _ := newPublicTestService()
	published := createPublished(t, svc, "Опублікована стаття")
	detail := requirePublishedDetail(t, svc)

	got, err := detail.GetPublishedBySlug(context.Background(), published.Slug)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, published.Slug, got.Slug)
	assert.Equal(t, models.StatusPublished, got.Status)
	assert.Equal(t, published.Text, got.Text)
}

func TestGetPublishedBySlug_RejectsDraft(t *testing.T) {
	svc, _, _ := newPublicTestService()
	draft, err := svc.Create(context.Background(), validInput())
	require.NoError(t, err)
	require.Equal(t, models.StatusDraft, draft.Status)
	detail := requirePublishedDetail(t, svc)

	got, err := detail.GetPublishedBySlug(context.Background(), draft.Slug)

	require.Error(t, err)
	assert.Nil(t, got, "a draft must not be returned to the public site")
	assert.True(t, errors.Is(err, errs.ErrNewsNotFound), "expected ErrNewsNotFound, got %v", err)
}

func TestGetPublishedBySlug_WrapsRepositoryError(t *testing.T) {
	cause := errors.New("db is down")
	svc, repo, handler := newPublicTestService()
	repo.GetErr = cause
	detail := requirePublishedDetail(t, svc)

	got, err := detail.GetPublishedBySlug(context.Background(), "any-article")

	require.Error(t, err)
	assert.Nil(t, got)
	assert.True(t, errors.Is(err, cause), "the repository cause must be preserved, got %v", err)
	assert.False(t, errors.Is(err, errs.ErrNewsNotFound),
		"an infrastructure failure must not be reported as a missing article")
	assert.True(t, handler.hasFC("newsService_GetPublishedBySlug"),
		"expected a log record carrying the fc identifier")
}
