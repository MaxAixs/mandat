package handler_test

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"testing"

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
	"featured", "views", "published_at", "text",
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
