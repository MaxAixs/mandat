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

// seedSlugArticle stores an article with an explicit slug, bypassing the
// service slug generation so collision layouts can be staged exactly.
func seedSlugArticle(repo *testsupport.StubRepo, slug, title string) *models.News {
	now := time.Now().UTC()
	id := uuid.New()
	news := &models.News{
		ID:        id,
		Slug:      slug,
		Category:  models.CategoryNews,
		Title:     title,
		Text:      "Повний текст",
		CreatedAt: now,
		UpdatedAt: now,
	}
	repo.Items[id] = news

	return news
}

func TestUpdate_KeepsSlugWhenTitleChanges(t *testing.T) {
	svc, repo := newTestService()
	ctx := context.Background()

	created, err := svc.Create(ctx, validInput())
	require.NoError(t, err)
	originalSlug := created.Slug

	update := validInput()
	update.Title = "Цілком інший заголовок"

	updated, err := svc.Update(ctx, originalSlug, update)

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, originalSlug, updated.Slug,
		"the slug is fixed at creation and must not follow a title change")
	assert.Equal(t, "Цілком інший заголовок", updated.Title)
	stored, err := repo.GetBySlug(ctx, originalSlug)
	require.NoError(t, err, "the article must stay reachable under its original slug")
	assert.Equal(t, originalSlug, stored.Slug)
}

func TestUpdate_SuffixedSlugRemainsStable(t *testing.T) {
	svc, repo := newTestService()
	ctx := context.Background()

	seedSlugArticle(repo, "foo", "Foo")
	existing := seedSlugArticle(repo, "foo-2", "Foo")

	update := validInput()
	update.Title = "Foo revised"

	updated, err := svc.Update(ctx, "foo-2", update)

	require.NoError(t, err,
		"updating an article whose slug carries a numeric suffix must succeed while foo exists")
	require.NotNil(t, updated)
	assert.Equal(t, "foo-2", updated.Slug,
		"a suffixed slug must be kept, not renamed or rejected as a conflict")
	assert.Equal(t, "Foo revised", updated.Title)
	assert.Equal(t, "foo-2", repo.Items[existing.ID].Slug)
}
