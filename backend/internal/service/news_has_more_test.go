package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/maxaixs/mandat/backend/internal/models"
	"github.com/maxaixs/mandat/backend/internal/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedListNews stores n drafts directly and returns them in slug order.
func seedListNews(repo *testsupport.StubRepo, n int) {
	now := time.Now().UTC()
	for i := 0; i < n; i++ {
		id := uuid.New()
		repo.Items[id] = &models.News{
			ID:        id,
			Slug:      string(rune('a'+i)) + "-article",
			Category:  models.CategoryNews,
			Title:     string(rune('a'+i)) + "-article",
			Text:      "Повний текст",
			CreatedAt: now,
			UpdatedAt: now,
		}
	}
}

func TestList_RequestsExtraRowForNextPage(t *testing.T) {
	tests := []struct {
		name       string
		page       int
		limit      int
		wantLimit  int
		wantOffset int
	}{
		{"first page", 1, 10, 11, 0},
		{"second page", 2, 10, 11, 10},
		{"third page small limit", 3, 2, 3, 4},
		{"clamped limit", 1, 100000, MaxPageLimit + 1, 0},
		{"clamped limit later page", 3, 100000, MaxPageLimit + 1, 2 * MaxPageLimit},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo := newTestService()

			result, err := svc.List(context.Background(), tt.page, tt.limit, "", "")

			require.NoError(t, err)
			assert.Equal(t, tt.wantLimit, repo.LastFilter.Limit,
				"the repository must receive one extra row to detect a next page")
			assert.Equal(t, tt.wantOffset, repo.LastFilter.Offset)
			assert.False(t, result.HasMore, "an empty repository has no next page")
		})
	}
}

func TestList_TrimsExtraRowAndSetsHasMore(t *testing.T) {
	svc, repo := newTestService()
	seedListNews(repo, 3)

	result, err := svc.List(context.Background(), 1, 2, "", "")

	require.NoError(t, err)
	require.Len(t, result.Items, 2, "the extra probe row must be trimmed from the result")
	assert.True(t, result.HasMore, "three rows with limit 2 must report a next page")
	assert.Equal(t, 1, result.Page)
	assert.Equal(t, 2, result.Limit)
}

func TestList_HasMoreFalseWhenFittingInLimit(t *testing.T) {
	tests := []struct {
		name   string
		stored int
		limit  int
	}{
		{"fewer than limit", 1, 2},
		{"exactly limit", 2, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo := newTestService()
			seedListNews(repo, tt.stored)

			result, err := svc.List(context.Background(), 1, tt.limit, "", "")

			require.NoError(t, err)
			assert.Len(t, result.Items, tt.stored)
			assert.False(t, result.HasMore)
		})
	}
}

func TestList_SecondPageHasMore(t *testing.T) {
	svc, repo := newTestService()
	seedListNews(repo, 5)

	result, err := svc.List(context.Background(), 2, 2, "", "")

	require.NoError(t, err)
	assert.Equal(t, 3, repo.LastFilter.Limit)
	assert.Equal(t, 2, repo.LastFilter.Offset)
	require.Len(t, result.Items, 2)
	assert.True(t, result.HasMore, "rows beyond the second page must report a next page")
}

func TestListPublished_RequestsExtraRowForNextPage(t *testing.T) {
	tests := []struct {
		name       string
		page       int
		limit      int
		wantLimit  int
		wantOffset int
	}{
		{"first page", 1, 10, 11, 0},
		{"second page", 2, 5, 6, 5},
		{"clamped limit", 2, 100000, MaxPageLimit + 1, MaxPageLimit},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo := newTestService()

			result, err := svc.ListPublished(context.Background(), tt.page, tt.limit, "")

			require.NoError(t, err)
			assert.Equal(t, tt.wantLimit, repo.LastPublishedFilter.Limit,
				"the repository must receive one extra row to detect a next page")
			assert.Equal(t, tt.wantOffset, repo.LastPublishedFilter.Offset)
			assert.False(t, result.HasMore, "an empty repository has no next page")
		})
	}
}

func TestListPublished_TrimsExtraRowAndSetsHasMore(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()

	createPublished(t, svc, "First published")
	createPublished(t, svc, "Second published")
	createPublished(t, svc, "Third published")

	result, err := svc.ListPublished(ctx, 1, 2, "")

	require.NoError(t, err)
	require.Len(t, result.Items, 2, "the extra probe row must be trimmed from the result")
	assert.True(t, result.HasMore, "three published rows with limit 2 must report a next page")
	assert.Equal(t, 1, result.Page)
	assert.Equal(t, 2, result.Limit)
}

func TestListPublished_HasMoreFalseWhenFittingInLimit(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()

	createPublished(t, svc, "Only published")

	result, err := svc.ListPublished(ctx, 1, 2, "")

	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	assert.False(t, result.HasMore)
}

func TestSearchPublished_AlwaysHasMoreFalse(t *testing.T) {
	repo := &searchFakeRepo{}
	svc := newSearchTestService(repo)
	ctx := context.Background()

	for _, title := range []string{"Уряд один", "Уряд два", "Уряд три"} {
		repo.items = append(repo.items, &models.News{Title: title, Status: models.StatusPublished})
	}

	result, err := svc.SearchPublished(ctx, "уряд", 2)

	require.NoError(t, err)
	assert.False(t, result.HasMore, "search is a single suggestion page and never reports a next page")
}
