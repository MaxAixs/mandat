package postgres_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/maxaixs/mandat/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// searchResultSlugs collects the slugs of search results in returned order.
func searchResultSlugs(items []*models.News) []string {
	slugs := make([]string, 0, len(items))
	for _, item := range items {
		slugs = append(slugs, item.Slug)
	}

	return slugs
}

// TestNewsRepo_SearchPublished pins the public live-search query: published
// titles only, newest publication first, wildcard characters matched
// literally, and LIMIT applied. It truncates the news table, so it runs only
// when DATABASE_URL points at a disposable test database.
func TestNewsRepo_SearchPublished(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()
	base := time.Date(2026, time.November, 3, 12, 0, 0, 0, time.UTC)

	// Two published articles share the word "Пошуков"; the draft matches too.
	older := publishedNews("search-older", base)
	older.Title = "Пошукова стаття про бюджет"
	newer := publishedNews("search-newer", base.Add(time.Hour))
	newer.Title = "Пошуковий огляд бюджету"
	draft := newsFixture("search-draft", models.StatusDraft, base.Add(2*time.Hour))
	draft.Title = "Пошукова чернетка про бюджет"

	// Literal wildcard characters in the title.
	percent := publishedNews("search-percent", base.Add(3*time.Hour))
	percent.Title = "Зростання на 15% у листопаді"
	underscored := publishedNews("search-underscore", base.Add(4*time.Hour))
	underscored.Title = "Файл news_2026 у релізі"

	// A literal backslash must survive the LIKE escape character.
	backslash := publishedNews("search-backslash", base.Add(5*time.Hour))
	backslash.Title = "Шлях C:\\новини"

	// Three further published articles for the LIMIT check.
	limitOldest := publishedNews("search-limit-oldest", base.Add(10*time.Hour))
	limitOldest.Title = "Ліміт перший"
	limitMiddle := publishedNews("search-limit-middle", base.Add(11*time.Hour))
	limitMiddle.Title = "Ліміт другий"
	limitNewest := publishedNews("search-limit-newest", base.Add(12*time.Hour))
	limitNewest.Title = "Ліміт третій"

	// Inserted out of publication order on purpose: results must be ordered by
	// published_at, not by insertion or creation.
	for _, news := range []*models.News{
		limitMiddle, newer, percent, older, draft, limitNewest, underscored, backslash, limitOldest,
	} {
		require.NoError(t, repo.Create(ctx, news), "seed %s", news.Slug)
	}

	t.Run("published titles newest first, drafts excluded", func(t *testing.T) {
		items, err := repo.SearchPublished(ctx, "Пошуков", 10)
		require.NoError(t, err)
		require.Len(t, items, 2, "only published articles may match; the matching draft must stay hidden")
		assert.Equal(t, []string{"search-newer", "search-older"}, searchResultSlugs(items),
			"newer publication must come first")
		for _, item := range items {
			assert.Equal(t, models.StatusPublished, item.Status)
		}
	})

	literalWildcardTests := []struct {
		name      string
		query     string
		wantSlugs []string
	}{
		{
			name:      "percent is matched literally, not as a wildcard",
			query:     "%",
			wantSlugs: []string{"search-percent"},
		},
		{
			name:      "underscore is matched literally, not as a wildcard",
			query:     "_",
			wantSlugs: []string{"search-underscore"},
		},
		{
			name:      "backslash is matched literally, not as the escape character",
			query:     "\\",
			wantSlugs: []string{"search-backslash"},
		},
		{
			name:      "common letter returns only titles containing it",
			query:     "б",
			wantSlugs: []string{"search-newer", "search-older"},
		},
	}

	for _, tt := range literalWildcardTests {
		t.Run(tt.name, func(t *testing.T) {
			items, err := repo.SearchPublished(ctx, tt.query, 10)
			require.NoError(t, err)
			assert.Equal(t, tt.wantSlugs, searchResultSlugs(items),
				"query %q must match titles literally", tt.query)
			for _, item := range items {
				assert.True(t, strings.Contains(item.Title, tt.query),
					"result %q must genuinely contain %q in its title", item.Slug, tt.query)
			}
		})
	}

	t.Run("limit is respected", func(t *testing.T) {
		items, err := repo.SearchPublished(ctx, "Ліміт", 2)
		require.NoError(t, err)
		require.Len(t, items, 2, "LIMIT 2 must cap three matching published articles")
		assert.Equal(t, []string{"search-limit-newest", "search-limit-middle"}, searchResultSlugs(items),
			"the surviving two must be the newest publications")
	})
}
