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
	"github.com/jackc/pgx/v5/pgconn"
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

	// CASCADE is required by news_viewers.news_id -> news(id).
	_, err = pool.Exec(ctx, "TRUNCATE news CASCADE")
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

func publishedMainEventIDs(t *testing.T, repo *newsrepo.NewsRepo) []uuid.UUID {
	t.Helper()

	items, err := repo.ListPublished(context.Background(), models.ListFilter{Page: 1, Limit: 100})
	require.NoError(t, err)

	var ids []uuid.UUID
	for _, item := range items {
		if item.MainEvent {
			ids = append(ids, item.ID)
		}
	}

	return ids
}

func requireSinglePublishedMainEvent(t *testing.T, repo *newsrepo.NewsRepo, wantID uuid.UUID) {
	t.Helper()

	ids := publishedMainEventIDs(t, repo)
	require.Len(t, ids, 1, "exactly one published article may remain the main event")
	assert.Equal(t, wantID, ids[0])
}

// pgUniqueViolationCode is the PostgreSQL SQLSTATE for unique constraint violations.
const pgUniqueViolationCode = "23505"

// pgCheckViolationCode is the PostgreSQL SQLSTATE for CHECK constraint violations.
const pgCheckViolationCode = "23514"

// newsCategoryCheckConstraint guards the fixed rubric list at the database layer.
const newsCategoryCheckConstraint = "news_category_check"

// indexNewsSinglePublishedMainEventPerCategory is the partial unique index that
// enforces one published main event per category.
const indexNewsSinglePublishedMainEventPerCategory = "idx_news_single_published_main_event_per_category"

func publishedMainEventIDsInCategory(
	t *testing.T,
	repo *newsrepo.NewsRepo,
	category models.Category,
) []uuid.UUID {
	t.Helper()

	items, err := repo.ListPublished(context.Background(), models.ListFilter{
		Category: &category,
		Page:     1,
		Limit:    100,
	})
	require.NoError(t, err)

	var ids []uuid.UUID
	for _, item := range items {
		if item.MainEvent {
			ids = append(ids, item.ID)
		}
	}

	return ids
}

// insertPublishedMainEventRaw seeds a row with raw SQL, bypassing the repository
// rotation. Needed while fixtures must hold mains in several categories at once.
func insertPublishedMainEventRaw(
	ctx context.Context,
	pool *pgxpool.Pool,
	news *models.News,
) error {
	query := `INSERT INTO news
		(id, slug, category, title, description, text, language, status,
		main_event, image_url, published_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`

	_, err := pool.Exec(ctx, query,
		news.ID, news.Slug, news.Category, news.Title, news.Description, news.Text,
		news.Language, news.Status, news.MainEvent, news.ImageUrl, news.PublishedAt,
		news.CreatedAt, news.UpdatedAt,
	)

	return err
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

func TestNewsRepo_Create_NewRubricsPassCategoryCheck(t *testing.T) {
	tests := []struct {
		name     string
		category models.Category
		slug     string
	}{
		{"politics", models.CategoryPolitics, "category-politics"},
		{"regional", models.CategoryRegional, "category-regional"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := setupRepo(t)
			ctx := context.Background()

			news := draftNews(tt.slug)
			news.Category = tt.category

			require.NoError(t, repo.Create(ctx, news),
				"the 00010 CHECK must accept the %q rubric", tt.category)

			got, err := repo.GetByID(ctx, news.ID)
			require.NoError(t, err)
			assert.Equal(t, tt.category, got.Category,
				"the stored article must keep the new rubric")
		})
	}
}

func TestNewsRepo_Create_UnknownRubricViolatesCategoryCheck(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	news := draftNews("category-fantastika")
	news.Category = models.Category("Фантастика")

	err := repo.Create(ctx, news)
	require.Error(t, err, "the 00010 CHECK must reject an unknown rubric")

	var pgErr *pgconn.PgError
	require.True(t, errors.As(err, &pgErr), "expected a PostgreSQL error, got %v", err)
	assert.Equal(t, pgCheckViolationCode, pgErr.Code)
	assert.Equal(t, newsCategoryCheckConstraint, pgErr.ConstraintName,
		"the rubric must be rejected by news_category_check")

	_, err = repo.GetBySlug(ctx, news.Slug)
	assert.True(t, errors.Is(err, errs.ErrNewsNotFound),
		"the rejected insert must not leave a row behind, got %v", err)
}

func TestNewsRepo_MainEventRoundTrip(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	news := draftNews("main-event-round-trip")
	news.MainEvent = true
	require.NoError(t, repo.Create(ctx, news))

	got, err := repo.GetByID(ctx, news.ID)
	require.NoError(t, err)
	assert.True(t, got.MainEvent, "create must persist main_event")

	news.MainEvent = false
	require.NoError(t, repo.Update(ctx, news))
	got, err = repo.GetByID(ctx, news.ID)
	require.NoError(t, err)
	assert.False(t, got.MainEvent, "update must persist clearing main_event")

	news.MainEvent = true
	require.NoError(t, repo.Update(ctx, news))
	got, err = repo.GetByID(ctx, news.ID)
	require.NoError(t, err)
	assert.True(t, got.MainEvent, "update must persist setting main_event")
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

func TestNewsRepo_Create_PublishedMainEventReplacesPrevious(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()
	base := time.Date(2026, time.October, 30, 12, 0, 0, 0, time.UTC)

	previous := publishedNews("previous-main-event", base)
	previous.MainEvent = true
	require.NoError(t, repo.Create(ctx, previous))

	replacement := publishedNews("replacement-main-event", base.Add(time.Hour))
	replacement.MainEvent = true
	require.NoError(t, repo.Create(ctx, replacement))

	requireSinglePublishedMainEvent(t, repo, replacement.ID)
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

func requireConcurrentMainEventWrites(
	t *testing.T,
	repo *newsrepo.NewsRepo,
	write func(context.Context, *models.News) error,
	items ...*models.News,
) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := make(chan struct{})
	results := make(chan error, len(items))
	for _, news := range items {
		go func(news *models.News) {
			<-start
			results <- write(ctx, news)
		}(news)
	}
	close(start)

	for i := range items {
		require.NoError(t, <-results, "concurrent published main-event write %d", i+1)
	}

	ids := publishedMainEventIDs(t, repo)
	require.Len(t, ids, 1, "concurrent transfers must leave exactly one published main-event row")
	assert.Contains(t, []uuid.UUID{items[0].ID, items[1].ID}, ids[0])
}

func TestNewsRepo_Create_ConcurrentPublishedMainEvent(t *testing.T) {
	repo := setupRepo(t)
	base := time.Date(2026, time.October, 30, 12, 0, 0, 0, time.UTC)

	first := publishedNews("concurrent-main-event-create-a", base)
	first.MainEvent = true
	second := publishedNews("concurrent-main-event-create-b", base.Add(time.Minute))
	second.MainEvent = true

	requireConcurrentMainEventWrites(t, repo, repo.Create, first, second)
}

func TestNewsRepo_Update_ConcurrentPublishedMainEvent(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()
	base := time.Date(2026, time.October, 30, 12, 0, 0, 0, time.UTC)

	first := publishedNews("concurrent-main-event-update-a", base)
	second := publishedNews("concurrent-main-event-update-b", base.Add(time.Minute))
	require.NoError(t, repo.Create(ctx, first))
	require.NoError(t, repo.Create(ctx, second))

	first.MainEvent = true
	second.MainEvent = true
	requireConcurrentMainEventWrites(t, repo, repo.Update, first, second)
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

func TestNewsRepo_PublishedMainEvent_DuplicateSlugRollsBack(t *testing.T) {
	tests := []struct {
		name string
		run  func(context.Context, *newsrepo.NewsRepo, *models.News, string) error
	}{
		{
			name: "create",
			run: func(ctx context.Context, repo *newsrepo.NewsRepo, candidate *models.News, conflictSlug string) error {
				attempted := publishedNews(conflictSlug, *candidate.PublishedAt)
				attempted.MainEvent = true
				return repo.Create(ctx, attempted)
			},
		},
		{
			name: "update",
			run: func(ctx context.Context, repo *newsrepo.NewsRepo, candidate *models.News, conflictSlug string) error {
				candidate.Slug = conflictSlug
				candidate.MainEvent = true
				return repo.Update(ctx, candidate)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := setupRepo(t)
			ctx := context.Background()
			base := time.Date(2026, time.October, 30, 12, 0, 0, 0, time.UTC)

			previous := publishedNews("previous-main-event", base)
			previous.MainEvent = true
			require.NoError(t, repo.Create(ctx, previous))

			conflict := newsFixture("occupied-main-event-slug", models.StatusDraft, base.Add(time.Hour))
			require.NoError(t, repo.Create(ctx, conflict))

			candidate := publishedNews("candidate-main-event-slug", base.Add(2*time.Hour))
			require.NoError(t, repo.Create(ctx, candidate))

			err := tt.run(ctx, repo, candidate, conflict.Slug)
			require.Error(t, err)
			assert.True(t, errors.Is(err, errs.ErrConflict), "expected ErrConflict, got %v", err)
			requireSinglePublishedMainEvent(t, repo, previous.ID)
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

func TestNewsRepo_List_CategoryFilterAppliesBeforePagination(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()
	base := time.Date(2026, time.October, 1, 12, 0, 0, 0, time.UTC)

	sportPublished := publishedNews("list-category-sport-published", base.Add(2*time.Hour))
	sportPublished.Category = models.CategoryInsides
	sportDraft := newsFixture("list-category-sport-draft", models.StatusDraft, base.Add(time.Hour))
	sportDraft.Category = models.CategoryInsides

	// Newer non-Sport rows would fill the first page if Category were applied after LIMIT/OFFSET.
	for i := 1; i <= 3; i++ {
		other := publishedNews(
			fmt.Sprintf("list-category-other-%02d", i),
			base.Add(time.Duration(i+2)*time.Hour),
		)
		other.Category = models.CategoryNews
		require.NoError(t, repo.Create(ctx, other))
	}
	require.NoError(t, repo.Create(ctx, sportDraft))
	require.NoError(t, repo.Create(ctx, sportPublished))

	sportCategory := models.CategoryInsides

	// Admin List does not restrict status, so Category-only returns both Sport rows.
	categoryOnly, err := repo.List(ctx, models.ListFilter{Category: &sportCategory, Page: 1, Limit: 20})
	require.NoError(t, err)
	require.Len(t, categoryOnly, 2, "Category-only List must include published and draft Sport rows")
	assert.Equal(t, "list-category-sport-published", categoryOnly[0].Slug)
	assert.Equal(t, "list-category-sport-draft", categoryOnly[1].Slug)
	assert.Equal(t, models.CategoryInsides, categoryOnly[0].Category)
	assert.Equal(t, models.CategoryInsides, categoryOnly[1].Category)

	published := models.StatusPublished
	publishedSport, err := repo.List(ctx, models.ListFilter{
		Status:   &published,
		Category: &sportCategory,
		Page:     1,
		Limit:    20,
	})
	require.NoError(t, err)
	require.Len(t, publishedSport, 1, "Status+Category List must exclude the Sport draft")
	assert.Equal(t, "list-category-sport-published", publishedSport[0].Slug)
	assert.Equal(t, models.CategoryInsides, publishedSport[0].Category)

	firstPage, err := repo.List(ctx, models.ListFilter{Category: &sportCategory, Page: 1, Limit: 1})
	require.NoError(t, err)
	require.Len(t, firstPage, 1, "page one must hold the newest Sport row, not a newer non-Sport row")
	assert.Equal(t, "list-category-sport-published", firstPage[0].Slug)

	secondPage, err := repo.List(ctx, models.ListFilter{Category: &sportCategory, Page: 2, Limit: 1})
	require.NoError(t, err)
	require.Len(t, secondPage, 1, "page two must keep the category filter")
	assert.Equal(t, "list-category-sport-draft", secondPage[0].Slug)

	thirdPage, err := repo.List(ctx, models.ListFilter{Category: &sportCategory, Page: 3, Limit: 1})
	require.NoError(t, err)
	assert.Empty(t, thirdPage)
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

func TestNewsRepo_ListPublished_OldMainEventLeadsFirstPage(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()
	base := time.Date(2026, time.October, 30, 12, 0, 0, 0, time.UTC)

	mainEvent := publishedNews("old-main-event", base.Add(-time.Hour))
	mainEvent.MainEvent = true
	require.NoError(t, repo.Create(ctx, mainEvent))

	featured := publishedNews("newer-featured", base)
	featured.Featured = true
	require.NoError(t, repo.Create(ctx, featured))

	for i := 1; i <= 21; i++ {
		news := publishedNews(fmt.Sprintf("main-event-regular-%02d", i), base.Add(time.Duration(i)*time.Minute))
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
		"old-main-event",
		"newer-featured",
		"main-event-regular-21",
		"main-event-regular-20",
		"main-event-regular-19",
		"main-event-regular-18",
		"main-event-regular-17",
		"main-event-regular-16",
		"main-event-regular-15",
		"main-event-regular-14",
		"main-event-regular-13",
		"main-event-regular-12",
		"main-event-regular-11",
		"main-event-regular-10",
		"main-event-regular-09",
		"main-event-regular-08",
		"main-event-regular-07",
		"main-event-regular-06",
		"main-event-regular-05",
		"main-event-regular-04",
	}, actualSlugs, "main event must lead page one, ahead of featured and newer chronological items")
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

func TestNewsRepo_Update_PublishedMainEventReplacesPrevious(t *testing.T) {
	tests := []struct {
		name             string
		initialStatus    models.Status
		initialMainEvent bool
	}{
		{
			name:          "published article becomes the main event",
			initialStatus: models.StatusPublished,
		},
		{
			name:             "main-event draft is published",
			initialStatus:    models.StatusDraft,
			initialMainEvent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := setupRepo(t)
			ctx := context.Background()
			base := time.Date(2026, time.October, 30, 12, 0, 0, 0, time.UTC)

			previous := publishedNews("previous-main-event", base)
			previous.MainEvent = true
			require.NoError(t, repo.Create(ctx, previous))

			target := newsFixture("update-main-event-target", tt.initialStatus, base.Add(time.Hour))
			target.MainEvent = tt.initialMainEvent
			require.NoError(t, repo.Create(ctx, target))
			requireSinglePublishedMainEvent(t, repo, previous.ID)

			target.Status = models.StatusPublished
			target.MainEvent = true
			target.UpdatedAt = base.Add(2 * time.Hour)
			if target.PublishedAt == nil {
				publishedAt := base.Add(2 * time.Hour)
				target.PublishedAt = &publishedAt
			}
			require.NoError(t, repo.Update(ctx, target))

			requireSinglePublishedMainEvent(t, repo, target.ID)
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
		_, err := pool.Exec(cleanupCtx, "TRUNCATE news CASCADE")
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

func TestNewsRepo_ListPublished_CategoryFilterAppliesBeforePagination(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()
	base := time.Date(2026, time.October, 1, 12, 0, 0, 0, time.UTC)

	sport := publishedNews("sport-older", base)
	sport.Category = models.CategoryInsides
	require.NoError(t, repo.Create(ctx, sport))

	// More unrelated articles than a page holds, all newer than the Sport item.
	for i := 1; i <= 25; i++ {
		newer := publishedNews(fmt.Sprintf("international-newer-%02d", i), base.Add(time.Duration(i)*time.Hour))
		newer.Category = models.CategoryAnalytics
		require.NoError(t, repo.Create(ctx, newer))
	}

	sportCategory := models.CategoryInsides
	items, err := repo.ListPublished(ctx, models.ListFilter{
		Category: &sportCategory,
		Page:     1,
		Limit:    20,
	})
	require.NoError(t, err)
	require.Len(t, items, 1, "category filtering must happen before LIMIT/OFFSET")
	assert.Equal(t, "sport-older", items[0].Slug)
	assert.Equal(t, models.CategoryInsides, items[0].Category)

	unfiltered, err := repo.ListPublished(ctx, models.ListFilter{Page: 1, Limit: 20})
	require.NoError(t, err)
	require.Len(t, unfiltered, 20)
	for _, item := range unfiltered {
		assert.NotEqual(t, models.CategoryInsides, item.Category,
			"the older Sport article must not appear among newer unrelated articles")
	}
}

// TestMainEventPerCategory pins the future per-category main-event rule:
// one published main per category, not one globally.
func TestMainEventPerCategory(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()
	base := time.Date(2026, time.November, 2, 12, 0, 0, 0, time.UTC)
	// Unique slugs keep the test independent of rows left by other runs.
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)

	sportMain := publishedNews("main-event-sport-"+suffix, base)
	sportMain.Category = models.CategoryInsides
	sportMain.MainEvent = true
	require.NoError(t, repo.Create(ctx, sportMain))

	newsMain := publishedNews("main-event-news-"+suffix, base.Add(time.Minute))
	newsMain.Category = models.CategoryNews
	newsMain.MainEvent = true
	require.NoError(t, repo.Create(ctx, newsMain))

	// Step 1: mains in different categories must coexist, so a main event in
	// one category must not clear the main event of another category.
	gotSport, err := repo.GetBySlug(ctx, sportMain.Slug)
	require.NoError(t, err)
	assert.True(t, gotSport.MainEvent,
		"a main event in category %q must survive a main event in another category", models.CategoryInsides)

	gotNews, err := repo.GetByID(ctx, newsMain.ID)
	require.NoError(t, err)
	assert.True(t, gotNews.MainEvent,
		"a main event in category %q must survive a main event in another category", models.CategoryNews)

	// Step 2: a second main event in the same category replaces the previous
	// one, and only within that category.
	secondSportMain := publishedNews("main-event-sport-second-"+suffix, base.Add(2*time.Minute))
	secondSportMain.Category = models.CategoryInsides
	secondSportMain.MainEvent = true
	require.NoError(t, repo.Create(ctx, secondSportMain))

	gotSport, err = repo.GetByID(ctx, sportMain.ID)
	require.NoError(t, err)
	assert.False(t, gotSport.MainEvent,
		"the previous main event of category %q must be cleared", models.CategoryInsides)

	gotSecondSport, err := repo.GetByID(ctx, secondSportMain.ID)
	require.NoError(t, err)
	assert.True(t, gotSecondSport.MainEvent,
		"the new main event of category %q must be set", models.CategoryInsides)

	gotNews, err = repo.GetBySlug(ctx, newsMain.Slug)
	require.NoError(t, err)
	assert.True(t, gotNews.MainEvent,
		"the main event of category %q must not be touched by a rotation in another category", models.CategoryNews)
}

func TestNewsRepo_ListPublished_CategoryFilterPaginatesMatchingRows(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()
	base := time.Date(2026, time.October, 1, 12, 0, 0, 0, time.UTC)

	sportSlugs := []string{"sport-1", "sport-2", "sport-3"}
	for i, slug := range sportSlugs {
		news := publishedNews(slug, base.Add(time.Duration(i)*time.Hour))
		news.Category = models.CategoryInsides
		require.NoError(t, repo.Create(ctx, news))
	}
	for i := 1; i <= 5; i++ {
		unrelated := publishedNews(fmt.Sprintf("unrelated-%02d", i), base.Add(time.Duration(i+10)*time.Hour))
		unrelated.Category = models.CategoryAnalytics
		require.NoError(t, repo.Create(ctx, unrelated))
	}

	sportCategory := models.CategoryInsides

	firstPage, err := repo.ListPublished(ctx, models.ListFilter{Category: &sportCategory, Page: 1, Limit: 2})
	require.NoError(t, err)
	require.Len(t, firstPage, 2)
	assert.Equal(t, "sport-3", firstPage[0].Slug)
	assert.Equal(t, "sport-2", firstPage[1].Slug)

	secondPage, err := repo.ListPublished(ctx, models.ListFilter{Category: &sportCategory, Page: 2, Limit: 2})
	require.NoError(t, err)
	require.Len(t, secondPage, 1)
	assert.Equal(t, "sport-1", secondPage[0].Slug)

	emptyPage, err := repo.ListPublished(ctx, models.ListFilter{Category: &sportCategory, Page: 3, Limit: 2})
	require.NoError(t, err)
	assert.Empty(t, emptyPage)
}

// TestMainEventRotationOnCategoryChange pins the rotation rule of a saved
// article: the destination category slot is cleared, the source category is
// left vacant, and unrelated categories keep their own main event.
func TestMainEventRotationOnCategoryChange(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()
	base := time.Date(2026, time.November, 2, 12, 0, 0, 0, time.UTC)

	sportMain := publishedNews("rotation-sport-main", base)
	sportMain.Category = models.CategoryInsides
	sportMain.MainEvent = true
	require.NoError(t, repo.Create(ctx, sportMain))

	// A main event of the destination category, set up before the move.
	newsMain := publishedNews("rotation-news-main", base.Add(time.Minute))
	newsMain.Category = models.CategoryNews
	newsMain.MainEvent = true
	require.NoError(t, repo.Create(ctx, newsMain))

	// Precondition: a main event in one category must not clear the main event
	// of another category.
	gotSportMain, err := repo.GetByID(ctx, sportMain.ID)
	require.NoError(t, err)
	require.True(t, gotSportMain.MainEvent,
		"the main event of %q must survive a main event in %q",
		models.CategoryInsides, models.CategoryNews)

	gotNewsMain, err := repo.GetByID(ctx, newsMain.ID)
	require.NoError(t, err)
	require.True(t, gotNewsMain.MainEvent,
		"the main event of %q must be set", models.CategoryNews)

	// Move the Sport main event into Новини in a single save, keeping the flag.
	sportMain.Category = models.CategoryNews
	sportMain.MainEvent = true
	sportMain.UpdatedAt = base.Add(2 * time.Hour)
	require.NoError(t, repo.Update(ctx, sportMain))

	assert.Empty(t, publishedMainEventIDsInCategory(t, repo, models.CategoryInsides),
		"the source category %q must be left without a published main event", models.CategoryInsides)
	assert.Equal(t, []uuid.UUID{sportMain.ID},
		publishedMainEventIDsInCategory(t, repo, models.CategoryNews),
		"the moved article must be the only main event of %q", models.CategoryNews)

	gotPreviousNewsMain, err := repo.GetByID(ctx, newsMain.ID)
	require.NoError(t, err)
	assert.False(t, gotPreviousNewsMain.MainEvent,
		"the previous main event of the destination category %q must be cleared", models.CategoryNews)

	gotMoved, err := repo.GetByID(ctx, sportMain.ID)
	require.NoError(t, err)
	assert.Equal(t, models.CategoryNews, gotMoved.Category)
	assert.True(t, gotMoved.MainEvent, "the moved article must stay the main event")
}

// TestMainEventDraftUpdateClearsNothing pins that a draft never triggers the
// published rotation: publishing is the moment when slots are exchanged.
func TestMainEventDraftUpdateClearsNothing(t *testing.T) {
	pool := setupPool(t)
	repo := newsrepo.NewNewsRepo(pool)
	ctx := context.Background()
	base := time.Date(2026, time.November, 2, 12, 0, 0, 0, time.UTC)

	// Seeded with raw SQL: the repository cannot place mains in two categories
	// at once while rotation is still global, and this test covers only drafts.
	sportMain := publishedNews("draft-update-sport-main", base)
	sportMain.Category = models.CategoryInsides
	sportMain.MainEvent = true
	require.NoError(t, insertPublishedMainEventRaw(ctx, pool, sportMain))

	newsMain := publishedNews("draft-update-news-main", base.Add(time.Minute))
	newsMain.Category = models.CategoryNews
	newsMain.MainEvent = true
	require.NoError(t, insertPublishedMainEventRaw(ctx, pool, newsMain))

	draft := newsFixture("draft-update-sport-draft", models.StatusDraft, base.Add(2*time.Minute))
	draft.Category = models.CategoryInsides
	require.NoError(t, repo.Create(ctx, draft))

	draft.MainEvent = true
	require.NoError(t, repo.Update(ctx, draft))

	gotSportMain, err := repo.GetByID(ctx, sportMain.ID)
	require.NoError(t, err)
	assert.True(t, gotSportMain.MainEvent,
		"a draft update must not clear the published main event of %q", models.CategoryInsides)

	gotNewsMain, err := repo.GetByID(ctx, newsMain.ID)
	require.NoError(t, err)
	assert.True(t, gotNewsMain.MainEvent,
		"a draft update must not clear the published main event of %q", models.CategoryNews)

	gotDraft, err := repo.GetByID(ctx, draft.ID)
	require.NoError(t, err)
	assert.True(t, gotDraft.MainEvent, "a draft keeps its main_event flag until published")

	assert.Equal(t, []uuid.UUID{sportMain.ID},
		publishedMainEventIDsInCategory(t, repo, models.CategoryInsides))
	assert.Equal(t, []uuid.UUID{newsMain.ID},
		publishedMainEventIDsInCategory(t, repo, models.CategoryNews))
}

// TestMainEventIndexBackstop checks the database-level invariant behind the
// repository rotation, bypassing the repository with raw SQL.
func TestMainEventIndexBackstop(t *testing.T) {
	base := time.Date(2026, time.November, 2, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name           string
		secondCategory models.Category
		wantConflict   bool
	}{
		{
			name:           "second published main event in the same category",
			secondCategory: models.CategoryInsides,
			wantConflict:   true,
		},
		{
			name:           "second published main event in another category",
			secondCategory: models.CategoryNews,
			wantConflict:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := setupPool(t)
			repo := newsrepo.NewNewsRepo(pool)
			ctx := context.Background()

			first := publishedNews("index-backstop-first", base)
			first.Category = models.CategoryInsides
			first.MainEvent = true
			require.NoError(t, repo.Create(ctx, first))

			second := publishedNews("index-backstop-second", base.Add(time.Minute))
			second.Category = tt.secondCategory
			second.MainEvent = true
			err := insertPublishedMainEventRaw(ctx, pool, second)

			if !tt.wantConflict {
				require.NoError(t, err,
					"the per-category index must allow a main event in another category")
				assert.Equal(t, []uuid.UUID{second.ID},
					publishedMainEventIDsInCategory(t, repo, tt.secondCategory))

				return
			}

			require.Error(t, err,
				"the index must reject a second published main event in the same category")

			var pgErr *pgconn.PgError
			require.True(t, errors.As(err, &pgErr), "expected a PostgreSQL error, got %v", err)
			assert.Equal(t, pgUniqueViolationCode, pgErr.Code)
			assert.Equal(t, indexNewsSinglePublishedMainEventPerCategory, pgErr.ConstraintName)

			_, err = repo.GetBySlug(ctx, second.Slug)
			assert.True(t, errors.Is(err, errs.ErrNewsNotFound),
				"the rejected insert must not leave a row behind, got %v", err)
			assert.Equal(t, []uuid.UUID{first.ID},
				publishedMainEventIDsInCategory(t, repo, models.CategoryInsides))
		})
	}
}

// TestFeaturedRotationStaysGlobal pins that featured stays a single global slot
// even when the competing article carries main_event = FALSE.
func TestFeaturedRotationStaysGlobal(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()
	base := time.Date(2026, time.November, 2, 12, 0, 0, 0, time.UTC)

	first := publishedNews("featured-global-first", base)
	first.Category = models.CategoryInsides
	first.Featured = true
	require.NoError(t, repo.Create(ctx, first))

	second := publishedNews("featured-global-second", base.Add(time.Minute))
	second.Category = models.CategoryNews
	second.Featured = true
	second.MainEvent = false
	require.NoError(t, repo.Create(ctx, second))

	gotFirst, err := repo.GetByID(ctx, first.ID)
	require.NoError(t, err)
	assert.False(t, gotFirst.Featured,
		"featured must stay global: a featured article in another category takes the slot")

	gotSecond, err := repo.GetByID(ctx, second.ID)
	require.NoError(t, err)
	assert.True(t, gotSecond.Featured)

	requireSinglePublishedFeatured(t, repo, second.ID)
}
