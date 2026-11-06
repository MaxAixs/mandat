package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// searchFakeRepo records every search call and returns canned results.
// It implements NewsRepository including the planned SearchPublished method.
type searchFakeRepo struct {
	items     []*models.News
	searchErr error

	searchCalls int
	lastQuery   string
	lastLimit   int
}

var _ NewsRepository = (*searchFakeRepo)(nil)

func (r *searchFakeRepo) Create(context.Context, *models.News) error { return nil }

func (r *searchFakeRepo) GetBySlug(context.Context, string) (*models.News, error) {
	return nil, errs.ErrNewsNotFound
}

func (r *searchFakeRepo) List(context.Context, models.ListFilter) ([]*models.News, error) {
	return nil, nil
}

func (r *searchFakeRepo) ListPublished(context.Context, models.ListFilter) ([]*models.News, error) {
	return nil, nil
}

func (r *searchFakeRepo) Update(context.Context, *models.News) error { return nil }

func (r *searchFakeRepo) Delete(context.Context, uuid.UUID) error { return nil }

func (r *searchFakeRepo) SearchPublished(_ context.Context, query string, limit int) ([]*models.News, error) {
	r.searchCalls++
	r.lastQuery = query
	r.lastLimit = limit
	if r.searchErr != nil {
		return nil, r.searchErr
	}

	return r.items, nil
}

func newSearchTestService(repo *searchFakeRepo) *NewsService {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	return NewNewsService(repo, logger, nil, "", nil)
}

func TestSearchPublished_EmptyQueryReturnsEmptyList(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{"empty string", ""},
		{"spaces only", "   "},
		{"mixed whitespace", "\t\n "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &searchFakeRepo{}
			svc := newSearchTestService(repo)

			result, err := svc.SearchPublished(context.Background(), tt.query, 7)

			require.NoError(t, err, "a blank query must not be an error: live suggestions clear the field")
			require.NotNil(t, result)
			require.NotNil(t, result.Items, "items must be a non-nil empty slice, not null JSON")
			assert.Empty(t, result.Items)
			assert.Equal(t, 0, repo.searchCalls, "a blank query must not reach the repository")
		})
	}
}

func TestSearchPublished_TrimsQuery(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  string
	}{
		{"surrounding spaces", "  уряд  ", "уряд"},
		{"leading tab", "\tбюджет", "бюджет"},
		{"trailing newline", "спорт\n", "спорт"},
		{"inner spaces preserved", "  уряд схвалив  ", "уряд схвалив"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &searchFakeRepo{}
			svc := newSearchTestService(repo)

			_, err := svc.SearchPublished(context.Background(), tt.query, 7)

			require.NoError(t, err)
			assert.Equal(t, 1, repo.searchCalls, "a non-blank query must reach the repository once")
			assert.Equal(t, tt.want, repo.lastQuery, "the repository must receive the trimmed query")
		})
	}
}

func TestSearchPublished_ClampsLimitAboveMax(t *testing.T) {
	repo := &searchFakeRepo{}
	svc := newSearchTestService(repo)

	result, err := svc.SearchPublished(context.Background(), "уряд", 99)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 20, repo.lastLimit, "a limit above the search maximum must be clamped to 20")
	assert.Equal(t, 20, result.Limit)
}

func TestSearchPublished_RejectsBadLimit(t *testing.T) {
	tests := []struct {
		name  string
		limit int
	}{
		{"zero", 0},
		{"negative", -3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &searchFakeRepo{}
			svc := newSearchTestService(repo)

			_, err := svc.SearchPublished(context.Background(), "уряд", tt.limit)

			require.Error(t, err)
			assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
			assert.Equal(t, 0, repo.searchCalls, "an invalid limit must not reach the repository")
		})
	}
}

func TestSearchPublished_RejectsQueryOver20Chars(t *testing.T) {
	t.Run("21 runes are rejected", func(t *testing.T) {
		repo := &searchFakeRepo{}
		svc := newSearchTestService(repo)

		// Ukrainian letters are multi-byte: this proves the cap counts runes, not bytes.
		query := strings.Repeat("я", 21)

		_, err := svc.SearchPublished(context.Background(), query, 7)

		require.Error(t, err)
		assert.True(t, errors.Is(err, errs.ErrInvalidInput), "expected ErrInvalidInput, got %v", err)
		assert.Equal(t, 0, repo.searchCalls, "an over-long query must not reach the repository")
	})

	t.Run("20 runes are accepted", func(t *testing.T) {
		repo := &searchFakeRepo{}
		svc := newSearchTestService(repo)

		query := strings.Repeat("я", 20)

		_, err := svc.SearchPublished(context.Background(), query, 7)

		require.NoError(t, err, "exactly 20 runes is still within the cap")
		assert.Equal(t, 1, repo.searchCalls)
	})
}

func TestSearchPublished_OrdersAndShapesResult(t *testing.T) {
	newer := &models.News{
		ID:     uuid.New(),
		Slug:   "novyi-biudzhet",
		Title:  "Уряд схвалив новий бюджет",
		Status: models.StatusPublished,
	}
	older := &models.News{
		ID:     uuid.New(),
		Slug:   "staryi-biudzhet",
		Title:  "Бюджет минулого року",
		Status: models.StatusPublished,
	}
	items := []*models.News{newer, older}

	tests := []struct {
		name      string
		limit     int
		wantLimit int
	}{
		{"echoes limit", 5, 5},
		{"echoes clamped limit", 99, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &searchFakeRepo{items: items}
			svc := newSearchTestService(repo)

			result, err := svc.SearchPublished(context.Background(), "бюджет", tt.limit)

			require.NoError(t, err)
			require.NotNil(t, result)
			require.Len(t, result.Items, 2)
			assert.Equal(t, newer, result.Items[0], "the repository order must be preserved, newest first")
			assert.Equal(t, older, result.Items[1])
			assert.Equal(t, 1, result.Page, "search is a single page of suggestions")
			assert.Equal(t, tt.wantLimit, result.Limit)
			assert.Equal(t, tt.wantLimit, repo.lastLimit, "the repository must receive the resolved limit")
		})
	}
}

func TestSearchPublished_ReturnsNonNilItemsWhenRepoReturnsNil(t *testing.T) {
	repo := &searchFakeRepo{items: nil}
	svc := newSearchTestService(repo)

	result, err := svc.SearchPublished(context.Background(), "нічого", 7)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotNil(t, result.Items, "items must stay non-nil so the JSON envelope carries []")
	assert.Empty(t, result.Items)
}

func TestSearchPublished_RejectsMalformedQuery(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{"invalid UTF-8", "\xff\xfe"},
		{"NUL byte", "а\u0000б"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &searchFakeRepo{}
			svc := newSearchTestService(repo)

			_, err := svc.SearchPublished(context.Background(), tt.query, 7)

			require.Error(t, err)
			assert.True(t, errors.Is(err, errs.ErrInvalidInput),
				"a query that cannot be sent to PostgreSQL must be a validation error, got %v", err)
			assert.Equal(t, 0, repo.searchCalls,
				"a malformed query must never reach the repository")
		})
	}
}

func TestSearchPublished_PropagatesRepoError(t *testing.T) {
	cause := errors.New("db is down")
	repo := &searchFakeRepo{searchErr: cause}
	svc := newSearchTestService(repo)

	_, err := svc.SearchPublished(context.Background(), "уряд", 7)

	require.Error(t, err)
	assert.True(t, errors.Is(err, cause), "the repository cause must survive wrapping, got %v", err)
	assert.False(t, errors.Is(err, errs.ErrInvalidInput),
		"a repository failure is not a validation failure")
}
