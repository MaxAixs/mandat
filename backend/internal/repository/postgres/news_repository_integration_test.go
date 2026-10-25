package postgres_test

import (
	"context"
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
	newsrepo "github.com/maxaixs/mandat/backend/internal/repository/postgres"
	"github.com/maxaixs/mandat/backend/migrations"
	pgdb "github.com/maxaixs/mandat/backend/pkg/database/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Integration tests run only when DATABASE_URL points at a real PostgreSQL
// (docker compose up -d db); otherwise they skip.

func databaseURL() string { return os.Getenv("DATABASE_URL") }

// testDBConfig parses the test DATABASE_URL into pool parameters.
func testDBConfig(t *testing.T) pgdb.DBConfig {
	t.Helper()
	if databaseURL() == "" {
		t.Skip("DATABASE_URL not set; skipping PostgreSQL integration test")
	}

	raw, err := url.Parse(databaseURL())
	require.NoError(t, err)

	password, _ := raw.User.Password()
	port, err := strconv.Atoi(raw.Port())
	if err != nil {
		t.Fatalf("parse DATABASE_URL port: %v", err)
	}

	sslmode := raw.Query().Get("sslmode")
	if sslmode == "" {
		sslmode = "disable"
	}

	return pgdb.DBConfig{
		Host:     raw.Hostname(),
		Port:     port,
		User:     raw.User.Username(),
		Password: password,
		Database: strings.TrimPrefix(raw.Path, "/"),
		SSLMode:  sslmode,
	}
}

func setupRepo(t *testing.T) *newsrepo.NewsRepo {
	t.Helper()
	dbCfg := testDBConfig(t)

	ctx := context.Background()
	pool, err := pgdb.NewPool(ctx, dbCfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	require.NoError(t, pgdb.Migrate(ctx, pool, migrations.FS))

	_, err = pool.Exec(ctx, "TRUNCATE news")
	require.NoError(t, err)

	return newsrepo.NewNewsRepo(pool)
}

func draftNews(slug string) *models.News {
	now := time.Now().UTC()
	return &models.News{
		ID:          uuid.New(),
		Slug:        slug,
		Category:    models.CategoryNews,
		Title:       "Тестовий заголовок",
		Description: "Короткий опис",
		Text:        "Повний текст",
		Language:    "uk",
		Status:      models.StatusDraft,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func TestNewsRepo_CreateAndGetByID(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	news := draftNews("uryad-skhalyv-biudzhet")
	news.Featured = true
	news.ImageUrl = "https://example.com/cover.jpg"
	require.NoError(t, repo.Create(ctx, news))

	assert.NotZero(t, news.ID)
	assert.False(t, news.CreatedAt.IsZero())
	assert.False(t, news.UpdatedAt.IsZero())

	got, err := repo.GetByID(ctx, news.ID)
	require.NoError(t, err)
	assert.Equal(t, news.ID, got.ID)
	assert.Equal(t, "uryad-skhalyv-biudzhet", got.Slug)

	bySlug, err := repo.GetBySlug(ctx, news.Slug)
	require.NoError(t, err)
	assert.Equal(t, news.ID, bySlug.ID)
	assert.Equal(t, models.CategoryNews, got.Category)
	assert.Equal(t, "Тестовий заголовок", got.Title)
	assert.Equal(t, "Короткий опис", got.Description)
	assert.Equal(t, "Повний текст", got.Text)
	assert.Equal(t, "uk", got.Language)
	assert.Equal(t, models.StatusDraft, got.Status)
	assert.Equal(t, int64(0), got.Views)
	assert.True(t, got.Featured)
	assert.Equal(t, "https://example.com/cover.jpg", got.ImageUrl)
	assert.Nil(t, got.PublishedAt)
}

func TestNewsRepo_Create_DuplicateSlug(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, draftNews("same-slug")))
	err := repo.Create(ctx, draftNews("same-slug"))

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrConflict), "expected ErrConflict, got %v", err)
}

func TestNewsRepo_Create_HostileStringsRoundTrip(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	hostile := draftNews("hostile'--slug")
	hostile.Title = `'); DROP TABLE news;--`
	require.NoError(t, repo.Create(ctx, hostile))

	got, err := repo.GetByID(ctx, hostile.ID)
	require.NoError(t, err)
	assert.Equal(t, `'); DROP TABLE news;--`, got.Title)
	assert.Equal(t, "hostile'--slug", got.Slug)

	// Table must still exist and be queryable.
	_, err = repo.List(ctx, models.ListFilter{Page: 1, Limit: 20})
	require.NoError(t, err)
}

func TestNewsRepo_GetByID_NotFound(t *testing.T) {
	repo := setupRepo(t)

	_, err := repo.GetByID(context.Background(), uuid.New())

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrNewsNotFound), "expected ErrNewsNotFound, got %v", err)
}

func TestNewsRepo_GetBySlug_NotFound(t *testing.T) {
	repo := setupRepo(t)

	_, err := repo.GetBySlug(context.Background(), "missing-article")

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrNewsNotFound), "expected ErrNewsNotFound, got %v", err)
}

func TestNewsRepo_List_PaginationAndFilter(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	published := models.StatusPublished
	for i, slug := range []string{"a1", "a2", "a3", "d1", "d2"} {
		n := draftNews(slug)
		if i < 3 {
			n.Status = models.StatusPublished
		}
		require.NoError(t, repo.Create(ctx, n))
	}

	items, err := repo.List(ctx, models.ListFilter{Page: 1, Limit: 2})
	require.NoError(t, err)
	assert.Len(t, items, 2)
	// Newest first: last created (d2) comes first.
	assert.Equal(t, "d2", items[0].Slug)
	assert.Equal(t, "d1", items[1].Slug)

	items, err = repo.List(ctx, models.ListFilter{Status: &published, Page: 1, Limit: 20})
	require.NoError(t, err)
	assert.Len(t, items, 3)

	items, err = repo.List(ctx, models.ListFilter{Page: 3, Limit: 2})
	require.NoError(t, err)
	assert.Len(t, items, 1)
}

func TestNewsRepo_ListPublished_ExcludesDraftsAndOrdersByPublishedAt(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	base := time.Date(2026, time.October, 1, 12, 0, 0, 0, time.UTC)

	olderPublication := draftNews("published-older")
	olderPublication.Status = models.StatusPublished
	olderPublishedAt := base.Add(-2 * time.Hour)
	olderPublication.PublishedAt = &olderPublishedAt
	// Deliberately the newest created_at: ordering must follow published_at.
	olderPublication.CreatedAt = base.Add(2 * time.Hour)
	olderPublication.UpdatedAt = base.Add(2 * time.Hour)

	newerPublication := draftNews("published-newer")
	newerPublication.Status = models.StatusPublished
	newerPublishedAt := base.Add(-1 * time.Hour)
	newerPublication.PublishedAt = &newerPublishedAt
	// Deliberately the oldest created_at: created_at ordering would invert the result.
	newerPublication.CreatedAt = base.Add(-2 * time.Hour)
	newerPublication.UpdatedAt = base.Add(-2 * time.Hour)

	draft := draftNews("draft-hidden")
	draft.CreatedAt = base.Add(3 * time.Hour)
	draft.UpdatedAt = base.Add(3 * time.Hour)

	for _, news := range []*models.News{draft, olderPublication, newerPublication} {
		require.NoError(t, repo.Create(ctx, news))
	}

	items, err := repo.ListPublished(ctx, models.ListFilter{Page: 1, Limit: 10})
	require.NoError(t, err)
	require.Len(t, items, 2, "drafts must not appear in the public list")
	assert.Equal(t, "published-newer", items[0].Slug, "newest publication must come first")
	assert.Equal(t, "published-older", items[1].Slug)
}

func TestNewsRepo_ListPublished_Pagination(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	base := time.Date(2026, time.October, 1, 12, 0, 0, 0, time.UTC)

	for i, slug := range []string{"first", "second"} {
		news := draftNews(slug)
		news.Status = models.StatusPublished
		publishedAt := base.Add(time.Duration(i) * time.Hour)
		news.PublishedAt = &publishedAt
		require.NoError(t, repo.Create(ctx, news))
	}

	firstPage, err := repo.ListPublished(ctx, models.ListFilter{Page: 1, Limit: 1})
	require.NoError(t, err)
	require.Len(t, firstPage, 1)
	assert.Equal(t, "second", firstPage[0].Slug)

	secondPage, err := repo.ListPublished(ctx, models.ListFilter{Page: 2, Limit: 1})
	require.NoError(t, err)
	require.Len(t, secondPage, 1)
	assert.Equal(t, "first", secondPage[0].Slug)

	emptyPage, err := repo.ListPublished(ctx, models.ListFilter{Page: 3, Limit: 1})
	require.NoError(t, err)
	assert.Empty(t, emptyPage)
}

func TestNewsRepo_Update(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	news := draftNews("to-update")
	require.NoError(t, repo.Create(ctx, news))
	createdAt := news.CreatedAt

	time.Sleep(1100 * time.Millisecond) // TIMESTAMPTZ resolution guard

	news.Title = "Оновлений заголовок"
	news.Status = models.StatusPublished
	now := time.Now().UTC()
	news.PublishedAt = &now
	news.UpdatedAt = now
	require.NoError(t, repo.Update(ctx, news))

	got, err := repo.GetByID(ctx, news.ID)
	require.NoError(t, err)
	assert.Equal(t, "Оновлений заголовок", got.Title)
	assert.Equal(t, models.StatusPublished, got.Status)
	require.NotNil(t, got.PublishedAt)
	assert.WithinDuration(t, createdAt, got.CreatedAt, time.Second, "CreatedAt must not change")
	assert.True(t, got.UpdatedAt.After(createdAt), "UpdatedAt must advance")
}

func TestNewsRepo_Update_PersistsUpdatedAt(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	news := draftNews("fresh-updated-at")
	require.NoError(t, repo.Create(ctx, news))
	createdAt := news.CreatedAt

	time.Sleep(1100 * time.Millisecond) // TIMESTAMPTZ resolution guard

	news.Title = "Змінений заголовок"
	news.UpdatedAt = time.Now().UTC()
	require.NoError(t, repo.Update(ctx, news))

	assert.True(t, news.UpdatedAt.After(createdAt),
		"UpdatedAt must advance, got %v (created at %v)", news.UpdatedAt, createdAt)

	got, err := repo.GetByID(ctx, news.ID)
	require.NoError(t, err)
	assert.WithinDuration(t, news.UpdatedAt, got.UpdatedAt, time.Second,
		"the caller-supplied UpdatedAt must reach the database")
}

func TestNewsRepo_Update_NotFound(t *testing.T) {
	repo := setupRepo(t)

	news := draftNews("ghost")
	news.ID = uuid.New()
	err := repo.Update(context.Background(), news)

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrNewsNotFound), "expected ErrNewsNotFound, got %v", err)
}

func TestNewsRepo_Delete(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	news := draftNews("to-delete")
	require.NoError(t, repo.Create(ctx, news))
	require.NoError(t, repo.Delete(ctx, news.ID))

	_, err := repo.GetByID(ctx, news.ID)
	assert.True(t, errors.Is(err, errs.ErrNewsNotFound))
}

func TestNewsRepo_Delete_NotFound(t *testing.T) {
	repo := setupRepo(t)

	err := repo.Delete(context.Background(), uuid.New())

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrNewsNotFound), "expected ErrNewsNotFound, got %v", err)
}
