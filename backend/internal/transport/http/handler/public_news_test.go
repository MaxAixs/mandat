package handler_test

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/maxaixs/mandat/backend/internal/models"
	"github.com/maxaixs/mandat/backend/internal/service"
	"github.com/maxaixs/mandat/backend/internal/testsupport"
	httptransport "github.com/maxaixs/mandat/backend/internal/transport/http"
	"github.com/maxaixs/mandat/backend/internal/transport/http/handler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const publishedBody = `{"category":"Міжнародні новини","title":"Опублікована стаття",` +
	`"description":"Стислий опис","text":"Повний текст","status":"published",` +
	`"featured":true,"image_url":"https://example.com/cover.jpg"}`

const publishedMainEventBody = `{"category":"Міжнародні новини","title":"Головна подія дня",` +
	`"description":"Стислий опис","text":"Повний текст","status":"published",` +
	`"main_event":true,"image_url":"https://example.com/main-event.jpg"}`

const publishedSportBody = `{"category":"Спорт","title":"Спортивна стаття",` +
	`"description":"Стислий опис","text":"Повний текст","status":"published",` +
	`"image_url":"https://example.com/sport.jpg"}`

const draftSportBody = `{"category":"Спорт","title":"Спортивна чернетка",` +
	`"description":"Стислий опис","text":"Повний текст"}`

type publicListPayload struct {
	Items []map[string]any `json:"items"`
	Page  int              `json:"page"`
	Limit int              `json:"limit"`
}

// setupPublicRouter assembles the admin and public news handlers over one router.
func setupPublicRouter() (http.Handler, *testsupport.StubRepo) {
	repo := testsupport.NewStubRepo()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := service.NewNewsService(repo, logger)

	httpSrv := httptransport.New(httptransport.HTTPConfig{
		AllowedOrigins: []string{allowedOrigin},
	}, handler.NewNewsHandler(svc), handler.NewPublicNewsHandler(svc))

	return httpSrv.Handler(), repo
}

func decodePublicList(t *testing.T, body []byte) publicListPayload {
	t.Helper()

	var payload publicListPayload
	require.NoError(t, json.Unmarshal(body, &payload))

	return payload
}

func TestPublicList_ReturnsOnlyPublishedArticles(t *testing.T) {
	router, _ := setupPublicRouter()
	createArticle(t, router, validBody) // draft
	publishedSlug := createArticle(t, router, publishedBody)

	rec := doRequest(t, router, http.MethodGet, "/api/news", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	payload := decodePublicList(t, rec.Body.Bytes())
	require.Len(t, payload.Items, 1)
	assert.Equal(t, publishedSlug, payload.Items[0]["slug"])
	assert.Equal(t, 1, payload.Page)
	assert.Equal(t, 20, payload.Limit)
}

func TestPublicList_ReturnsPublicFieldsOnly(t *testing.T) {
	router, _ := setupPublicRouter()
	slug := createArticle(t, router, publishedBody)

	rec := doRequest(t, router, http.MethodGet, "/api/news", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	payload := decodePublicList(t, rec.Body.Bytes())
	require.Len(t, payload.Items, 1)
	item := payload.Items[0]

	assert.Equal(t, slug, item["slug"])
	assert.Equal(t, "Міжнародні новини", item["category"])
	assert.Equal(t, "Опублікована стаття", item["title"])
	assert.Equal(t, "Стислий опис", item["description"])
	assert.Equal(t, "https://example.com/cover.jpg", item["image_url"])
	assert.Equal(t, true, item["featured"])
	assert.Equal(t, float64(0), item["views"])
	assert.NotEmpty(t, item["published_at"])

	for _, field := range []string{"id", "text", "status", "language", "created_at", "updated_at"} {
		assert.NotContains(t, item, field, "public payload must not expose %q", field)
	}
}

func TestPublicList_ReturnsMainEvent(t *testing.T) {
	router, _ := setupPublicRouter()
	slug := createArticle(t, router, publishedMainEventBody)

	rec := doRequest(t, router, http.MethodGet, "/api/news", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	payload := decodePublicList(t, rec.Body.Bytes())
	require.Len(t, payload.Items, 1)
	assert.Equal(t, slug, payload.Items[0]["slug"])
	assert.Equal(t, true, payload.Items[0]["main_event"])
}

func TestPublicList_IgnoresStatusQuery(t *testing.T) {
	router, _ := setupPublicRouter()
	createArticle(t, router, validBody) // draft
	publishedSlug := createArticle(t, router, publishedBody)

	rec := doRequest(t, router, http.MethodGet, "/api/news?status=draft", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	payload := decodePublicList(t, rec.Body.Bytes())
	require.Len(t, payload.Items, 1, "the public list must ignore a client-supplied status filter")
	assert.Equal(t, publishedSlug, payload.Items[0]["slug"])
}

func TestPublicList_DraftOnlyReturnsEmptyList(t *testing.T) {
	router, _ := setupPublicRouter()
	createArticle(t, router, validBody)

	rec := doRequest(t, router, http.MethodGet, "/api/news", "")

	require.Equal(t, http.StatusOK, rec.Code)
	payload := decodePublicList(t, rec.Body.Bytes())
	require.NotNil(t, payload.Items, "an empty page must serialize as [] rather than null")
	assert.Empty(t, payload.Items)
}

func TestPublicList_InvalidPaginationReturns400(t *testing.T) {
	router, _ := setupPublicRouter()

	for _, query := range []string{"?page=abc", "?page=0", "?page=-1", "?limit=abc", "?limit=0", "?limit=-1"} {
		rec := doRequest(t, router, http.MethodGet, "/api/news"+query, "")

		require.Equal(t, http.StatusBadRequest, rec.Code, query)
		assert.Equal(t, "Invalid input", decodeError(t, rec.Body.Bytes()).Error.Message, query)
	}
}

func TestPublicList_ClampsLimit(t *testing.T) {
	router, _ := setupPublicRouter()

	rec := doRequest(t, router, http.MethodGet, "/api/news?limit=1000000", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	payload := decodePublicList(t, rec.Body.Bytes())
	assert.Equal(t, 100, payload.Limit)
}

func TestPublicList_CategoryFilterReturnsOnlyMatchingPublishedArticles(t *testing.T) {
	router, _ := setupPublicRouter()
	sportSlug := createArticle(t, router, publishedSportBody)
	createArticle(t, router, publishedBody) // international
	createArticle(t, router, draftSportBody)

	rec := doRequest(t, router, http.MethodGet,
		"/api/news?category="+url.QueryEscape(string(models.CategoryInsides)), "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	payload := decodePublicList(t, rec.Body.Bytes())
	require.Len(t, payload.Items, 1, "only published articles of the requested category must be listed")
	assert.Equal(t, sportSlug, payload.Items[0]["slug"])
	assert.Equal(t, string(models.CategoryInsides), payload.Items[0]["category"])
}

func TestPublicList_CategoryFilterMatchesExactRubric(t *testing.T) {
	router, _ := setupPublicRouter()
	createArticle(t, router, publishedSportBody)
	internationalSlug := createArticle(t, router, publishedBody)

	rec := doRequest(t, router, http.MethodGet,
		"/api/news?category="+url.QueryEscape(string(models.CategoryAnalytics)), "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	payload := decodePublicList(t, rec.Body.Bytes())
	require.Len(t, payload.Items, 1)
	assert.Equal(t, internationalSlug, payload.Items[0]["slug"])
	assert.Equal(t, string(models.CategoryAnalytics), payload.Items[0]["category"])
}

func TestPublicList_WithoutCategoryRemainsUnfiltered(t *testing.T) {
	router, _ := setupPublicRouter()
	sportSlug := createArticle(t, router, publishedSportBody)
	internationalSlug := createArticle(t, router, publishedBody)
	createArticle(t, router, draftSportBody)

	rec := doRequest(t, router, http.MethodGet, "/api/news", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	payload := decodePublicList(t, rec.Body.Bytes())
	require.Len(t, payload.Items, 2, "the general feed must keep returning every published article")

	slugs := map[string]bool{}
	for _, item := range payload.Items {
		slug, ok := item["slug"].(string)
		require.True(t, ok)
		slugs[slug] = true
	}
	assert.True(t, slugs[sportSlug])
	assert.True(t, slugs[internationalSlug])
}

func TestPublicList_UnknownCategoryReturns400(t *testing.T) {
	tests := []struct {
		name     string
		category string
	}{
		{"unknown rubric", "Політика"},
		{"lowercase sport", "спорт"},
		{"trailing space", "Спорт "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, _ := setupPublicRouter()

			rec := doRequest(t, router, http.MethodGet,
				"/api/news?category="+url.QueryEscape(tt.category), "")

			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			assert.Equal(t, "Invalid input", decodeError(t, rec.Body.Bytes()).Error.Message)
		})
	}
}

// storePublished inserts a deterministic published article directly into the stub.
func storePublished(
	t *testing.T,
	repo *testsupport.StubRepo,
	slug string,
	category models.Category,
	publishedAt time.Time,
) {
	t.Helper()

	id := uuid.New()
	repo.Items[id] = &models.News{
		ID:          id,
		Slug:        slug,
		Category:    category,
		Title:       slug,
		Description: "Стислий опис",
		Text:        "Повний текст",
		Status:      models.StatusPublished,
		PublishedAt: &publishedAt,
		CreatedAt:   publishedAt,
		UpdatedAt:   publishedAt,
	}
}

func TestPublicList_CategoryFilterAppliesBeforePagination(t *testing.T) {
	router, repo := setupPublicRouter()
	base := time.Date(2026, time.October, 1, 12, 0, 0, 0, time.UTC)

	storePublished(t, repo, "sport-older", models.CategoryInsides, base)
	storePublished(t, repo, "international-newer", models.CategoryAnalytics, base.Add(time.Hour))

	unfiltered := doRequest(t, router, http.MethodGet, "/api/news?limit=1", "")
	require.Equal(t, http.StatusOK, unfiltered.Code, unfiltered.Body.String())
	unfilteredPayload := decodePublicList(t, unfiltered.Body.Bytes())
	require.Len(t, unfilteredPayload.Items, 1)
	assert.Equal(t, "international-newer", unfilteredPayload.Items[0]["slug"],
		"the newer unrelated article must lead the unfiltered first page")

	filtered := doRequest(t, router, http.MethodGet,
		"/api/news?limit=1&category="+url.QueryEscape(string(models.CategoryInsides)), "")
	require.Equal(t, http.StatusOK, filtered.Code, filtered.Body.String())
	filteredPayload := decodePublicList(t, filtered.Body.Bytes())
	require.Len(t, filteredPayload.Items, 1,
		"the older matching article must survive the first filtered page")
	assert.Equal(t, "sport-older", filteredPayload.Items[0]["slug"])
}

func TestPublicList_RepoFailureReturnsGeneric500(t *testing.T) {
	router, repo := setupPublicRouter()
	repo.ListPublishedErr = errors.New(`pq: relation "news" does not exist`)

	rec := doRequest(t, router, http.MethodGet, "/api/news", "")

	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
	body := decodeError(t, rec.Body.Bytes())
	assert.Equal(t, http.StatusInternalServerError, body.Error.Status)
	assert.Equal(t, "Internal server error", body.Error.Message)
	assert.NotContains(t, rec.Body.String(), "relation",
		"the response must not leak repository details")
}

const publishedDetailBody = `{"category":"Міжнародні новини","title":"Опублікована стаття",` +
	`"description":"Стислий опис","text":"Перший абзац.\nДругий абзац.",` +
	`"status":"published","featured":true,"image_url":"https://example.com/cover.jpg"}`

// publicDetailFields is the exact field set of a public detail payload.
var publicDetailFields = []string{
	"slug", "category", "title", "description", "image_url",
	"featured", "main_event", "views", "published_at", "text",
}

func decodePublicDetail(t *testing.T, body []byte) map[string]any {
	t.Helper()

	var payload map[string]any
	require.NoError(t, json.Unmarshal(body, &payload))

	return payload
}

func TestPublicDetail_ReturnsPublishedArticle(t *testing.T) {
	router, _ := setupPublicRouter()
	slug := createArticle(t, router, publishedDetailBody)

	rec := doRequest(t, router, http.MethodGet, "/api/news/"+slug, "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	detail := decodePublicDetail(t, rec.Body.Bytes())

	assert.Equal(t, slug, detail["slug"])
	assert.Equal(t, "Міжнародні новини", detail["category"])
	assert.Equal(t, "Опублікована стаття", detail["title"])
	assert.Equal(t, "Стислий опис", detail["description"])
	assert.Equal(t, "https://example.com/cover.jpg", detail["image_url"])
	assert.Equal(t, true, detail["featured"])
	assert.Equal(t, float64(0), detail["views"])
	assert.NotEmpty(t, detail["published_at"])
	assert.Equal(t, "Перший абзац.\nДругий абзац.", detail["text"],
		"multiline text must survive the response unchanged")

	require.Len(t, detail, len(publicDetailFields), "unexpected fields in %v", detail)
	for _, field := range publicDetailFields {
		assert.Contains(t, detail, field)
	}
	for _, field := range []string{"id", "status", "language", "created_at", "updated_at"} {
		assert.NotContains(t, detail, field, "public detail must not expose %q", field)
	}
}

func TestPublicDetail_ReturnsMainEvent(t *testing.T) {
	router, _ := setupPublicRouter()
	slug := createArticle(t, router, publishedMainEventBody)

	rec := doRequest(t, router, http.MethodGet, "/api/news/"+slug, "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	detail := decodePublicDetail(t, rec.Body.Bytes())
	assert.Equal(t, slug, detail["slug"])
	assert.Equal(t, true, detail["main_event"])
}

func TestPublicDetail_DraftAndMissingReturn404(t *testing.T) {
	router, _ := setupPublicRouter()
	draftSlug := createArticle(t, router, validBody)

	draftRec := doRequest(t, router, http.MethodGet, "/api/news/"+draftSlug, "")
	missingRec := doRequest(t, router, http.MethodGet, "/api/news/missing-article", "")

	require.Equal(t, http.StatusNotFound, draftRec.Code, draftRec.Body.String())
	require.Equal(t, http.StatusNotFound, missingRec.Code, missingRec.Body.String())

	draftBody := decodeError(t, draftRec.Body.Bytes())
	assert.Equal(t, http.StatusNotFound, draftBody.Error.Status)
	assert.Equal(t, "News article not found", draftBody.Error.Message)

	assert.Equal(t, draftRec.Body.String(), missingRec.Body.String(),
		"a guessed draft slug must be indistinguishable from a missing article")
}

func TestPublicDetail_RepoFailureReturnsGeneric500(t *testing.T) {
	router, repo := setupPublicRouter()
	repo.GetErr = errors.New(`pq: relation "news" does not exist`)

	rec := doRequest(t, router, http.MethodGet, "/api/news/missing-article", "")

	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
	body := decodeError(t, rec.Body.Bytes())
	assert.Equal(t, http.StatusInternalServerError, body.Error.Status)
	assert.Equal(t, "Internal server error", body.Error.Message)
	assert.NotContains(t, rec.Body.String(), "relation",
		"the response must not leak repository details")
}
