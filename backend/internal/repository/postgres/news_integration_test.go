package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
	newsrepo "github.com/maxaixs/mandat/backend/internal/repository/postgres"
	"github.com/maxaixs/mandat/backend/migrations"
	pgdb "github.com/maxaixs/mandat/backend/pkg/database/postgres"
	"github.com/pressly/goose/v3"
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

func setupPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dbCfg := testDBConfig(t)

	ctx := context.Background()
	pool, err := pgdb.NewPool(ctx, dbCfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	require.NoError(t, pgdb.Migrate(ctx, pool, migrations.FS))

	_, err = pool.Exec(ctx, "TRUNCATE news")
	require.NoError(t, err)

	return pool
}

func setupRepo(t *testing.T) *newsrepo.NewsRepo {
	t.Helper()

	return newsrepo.NewNewsRepo(setupPool(t))
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

func newsFixture(slug string, status models.Status, timestamp time.Time) *models.News {
	news := draftNews(slug)
	news.Status = status
	news.CreatedAt = timestamp
	news.UpdatedAt = timestamp
	if status == models.StatusPublished {
		news.PublishedAt = &timestamp
	}

	return news
}

func publishedNews(slug string, publishedAt time.Time) *models.News {
	return newsFixture(slug, models.StatusPublished, publishedAt)
}

func publishedFeaturedIDs(t *testing.T, repo *newsrepo.NewsRepo) []uuid.UUID {
	t.Helper()

	items, err := repo.ListPublished(context.Background(), models.ListFilter{Page: 1, Limit: 100})
	require.NoError(t, err)

	var ids []uuid.UUID
	for _, item := range items {
		if item.Featured {
			ids = append(ids, item.ID)
		}
	}

	return ids
}

func requireSinglePublishedFeatured(t *testing.T, repo *newsrepo.NewsRepo, wantID uuid.UUID) {
	t.Helper()

	ids := publishedFeaturedIDs(t, repo)
	require.Len(t, ids, 1, "exactly one published article may remain featured")
	assert.Equal(t, wantID, ids[0])
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

func TestNewsRepo_Create_PublishedFeaturedReplacesPrevious(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()
	base := time.Date(2026, time.October, 26, 12, 0, 0, 0, time.UTC)

	previous := publishedNews("previous-featured", base)
	previous.Featured = true
	require.NoError(t, repo.Create(ctx, previous))

	replacement := publishedNews("replacement-featured", base.Add(time.Hour))
	replacement.Featured = true
	require.NoError(t, repo.Create(ctx, replacement))

	requireSinglePublishedFeatured(t, repo, replacement.ID)
}

func TestNewsRepo_Create_ConcurrentPublishedFeatured(t *testing.T) {
	repo := setupRepo(t)
	base := time.Date(2026, time.October, 26, 12, 0, 0, 0, time.UTC)

	first := publishedNews("concurrent-featured-a", base)
	first.Featured = true
	second := publishedNews("concurrent-featured-b", base.Add(time.Minute))
	second.Featured = true

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := make(chan struct{})
	results := make(chan error, 2)
	for _, news := range []*models.News{first, second} {
		go func(news *models.News) {
			<-start
			results <- repo.Create(ctx, news)
		}(news)
	}
	close(start)

	var createErrors []error
	for i := 0; i < 2; i++ {
		createErrors = append(createErrors, <-results)
	}
	for i, err := range createErrors {
		require.NoError(t, err, "concurrent published-featured create %d", i+1)
	}

	ids := publishedFeaturedIDs(t, repo)
	require.Len(t, ids, 1, "concurrent transfers must leave exactly one published-featured row")
	assert.Contains(t, []uuid.UUID{first.ID, second.ID}, ids[0])
}

func TestNewsRepo_PublishedFeatured_DuplicateSlugRollsBack(t *testing.T) {
	tests := []struct {
		name string
		run  func(context.Context, *newsrepo.NewsRepo, *models.News, string) error
	}{
		{
			name: "create",
			run: func(ctx context.Context, repo *newsrepo.NewsRepo, candidate *models.News, conflictSlug string) error {
				attempted := publishedNews(conflictSlug, *candidate.PublishedAt)
				attempted.Featured = true
				return repo.Create(ctx, attempted)
			},
		},
		{
			name: "update",
			run: func(ctx context.Context, repo *newsrepo.NewsRepo, candidate *models.News, conflictSlug string) error {
				candidate.Slug = conflictSlug
				candidate.Featured = true
				return repo.Update(ctx, candidate)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := setupRepo(t)
			ctx := context.Background()
			base := time.Date(2026, time.October, 26, 12, 0, 0, 0, time.UTC)

			previous := publishedNews("previous-featured", base)
			previous.Featured = true
			require.NoError(t, repo.Create(ctx, previous))

			conflict := newsFixture("occupied-slug", models.StatusDraft, base.Add(time.Hour))
			require.NoError(t, repo.Create(ctx, conflict))

			candidate := publishedNews("candidate-slug", base.Add(2*time.Hour))
			require.NoError(t, repo.Create(ctx, candidate))

			err := tt.run(ctx, repo, candidate, conflict.Slug)
			require.Error(t, err)
			assert.True(t, errors.Is(err, errs.ErrConflict), "expected ErrConflict, got %v", err)
			requireSinglePublishedFeatured(t, repo, previous.ID)
		})
	}
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

func TestNewsRepo_ListPublished_OldFeaturedLeadsFirstPage(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()
	base := time.Date(2026, time.October, 26, 12, 0, 0, 0, time.UTC)

	featured := publishedNews("old-featured", base.Add(-time.Hour))
	featured.Featured = true
	require.NoError(t, repo.Create(ctx, featured))

	for i := 1; i <= 21; i++ {
		news := publishedNews(fmt.Sprintf("regular-%02d", i), base.Add(time.Duration(i)*time.Minute))
		require.NoError(t, repo.Create(ctx, news))
	}

	items, err := repo.ListPublished(ctx, models.ListFilter{Page: 1, Limit: 20})
	require.NoError(t, err)
	require.Len(t, items, 20)

	actualSlugs := make([]string, 0, len(items))
	for _, item := range items {
		actualSlugs = append(actualSlugs, item.Slug)
	}
	assert.Equal(t, []string{
		"old-featured",
		"regular-21",
		"regular-20",
		"regular-19",
		"regular-18",
		"regular-17",
		"regular-16",
		"regular-15",
		"regular-14",
		"regular-13",
		"regular-12",
		"regular-11",
		"regular-10",
		"regular-09",
		"regular-08",
		"regular-07",
		"regular-06",
		"regular-05",
		"regular-04",
		"regular-03",
	}, actualSlugs, "featured must lead page one and all remaining items must stay chronological")
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

func TestNewsRepo_Update_PublishedFeaturedReplacesPrevious(t *testing.T) {
	tests := []struct {
		name            string
		initialStatus   models.Status
		initialFeatured bool
	}{
		{
			name:          "published article becomes featured",
			initialStatus: models.StatusPublished,
		},
		{
			name:            "featured draft is published",
			initialStatus:   models.StatusDraft,
			initialFeatured: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := setupRepo(t)
			ctx := context.Background()
			base := time.Date(2026, time.October, 26, 12, 0, 0, 0, time.UTC)

			previous := publishedNews("previous-featured", base)
			previous.Featured = true
			require.NoError(t, repo.Create(ctx, previous))

			target := newsFixture("update-featured-target", tt.initialStatus, base.Add(time.Hour))
			target.Featured = tt.initialFeatured
			require.NoError(t, repo.Create(ctx, target))
			requireSinglePublishedFeatured(t, repo, previous.ID)

			target.Status = models.StatusPublished
			target.Featured = true
			target.UpdatedAt = base.Add(2 * time.Hour)
			if target.PublishedAt == nil {
				publishedAt := base.Add(2 * time.Hour)
				target.PublishedAt = &publishedAt
			}
			require.NoError(t, repo.Update(ctx, target))

			requireSinglePublishedFeatured(t, repo, target.ID)
		})
	}
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

func TestNewsRepo_Migration_DeduplicatesPublishedFeatured(t *testing.T) {
	pool := setupPool(t)
	ctx := context.Background()

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		if err := pgdb.Migrate(cleanupCtx, pool, migrations.FS); err != nil {
			t.Errorf("restore migrated state: %v", err)
			return
		}
		_, err := pool.Exec(cleanupCtx, "TRUNCATE news")
		assert.NoError(t, err, "clean up migration fixtures")
	})

	migrationDB := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = migrationDB.Close() })

	goose.SetBaseFS(migrations.FS)
	require.NoError(t, goose.SetDialect("postgres"))
	require.NoError(t, goose.DownToContext(ctx, migrationDB, ".", 1))

	base := time.Date(2026, time.October, 26, 12, 0, 0, 0, time.UTC)
	latestPublication := base.Add(2 * time.Hour)
	latestCreation := base.Add(2 * time.Hour)
	winnerID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	legacyRows := []struct {
		id          uuid.UUID
		slug        string
		publishedAt time.Time
		createdAt   time.Time
	}{
		{
			id:          uuid.MustParse("00000000-0000-0000-0000-000000000004"),
			slug:        "legacy-older-publication",
			publishedAt: base,
			createdAt:   base.Add(3 * time.Hour),
		},
		{
			id:          uuid.MustParse("00000000-0000-0000-0000-000000000003"),
			slug:        "legacy-latest-older-creation",
			publishedAt: latestPublication,
			createdAt:   base.Add(time.Hour),
		},
		{
			id:          uuid.MustParse("00000000-0000-0000-0000-000000000001"),
			slug:        "legacy-latest-tie-low-id",
			publishedAt: latestPublication,
			createdAt:   latestCreation,
		},
		{
			id:          winnerID,
			slug:        "legacy-latest-tie-high-id",
			publishedAt: latestPublication,
			createdAt:   latestCreation,
		},
	}

	for _, row := range legacyRows {
		_, err := pool.Exec(ctx, `INSERT INTO news
			(id, slug, category, title, status, featured, published_at, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, TRUE, $6, $7, $7)`,
			row.id, row.slug, models.CategoryNews, "Legacy duplicate", models.StatusPublished,
			row.publishedAt, row.createdAt,
		)
		require.NoError(t, err, "insert legacy duplicate %s", row.slug)
	}

	require.NoError(t, goose.UpContext(ctx, migrationDB, "."))

	repo := newsrepo.NewNewsRepo(pool)
	ids := publishedFeaturedIDs(t, repo)
	require.Len(t, ids, 1, "migration must deduplicate published-featured rows")
	assert.Equal(t, winnerID, ids[0], "latest publication, creation time, and ID must determine the retained row")
}
