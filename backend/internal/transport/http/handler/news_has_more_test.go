package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/maxaixs/mandat/backend/internal/models"
)

// hasMoreListPayload decodes a paginated list envelope including has_more.
type hasMoreListPayload struct {
	Items   []map[string]any `json:"items"`
	Page    int              `json:"page"`
	Limit   int              `json:"limit"`
	HasMore bool             `json:"has_more"`
}

func decodeHasMoreList(t *testing.T, body []byte) hasMoreListPayload {
	t.Helper()

	var payload hasMoreListPayload
	require.NoError(t, json.Unmarshal(body, &payload))

	return payload
}

// seedAdminArticles creates n articles with distinct titles through the admin API.
func seedAdminArticles(t *testing.T, router http.Handler, n int) {
	t.Helper()

	for i := 0; i < n; i++ {
		body := fmt.Sprintf(
			`{"category":"Новини","title":"Стаття номер %d","text":"Повний текст"}`,
			i+1,
		)
		rec := doRequest(t, router, http.MethodPost, "/api/admin/news", body)
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	}
}

func TestAdminList_HasMoreReflectsNextPage(t *testing.T) {
	router, _ := setupRouter(t)
	seedAdminArticles(t, router, 3)

	rec := doRequest(t, router, http.MethodGet, "/api/admin/news?limit=2", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	payload := decodeHasMoreList(t, rec.Body.Bytes())
	require.Len(t, payload.Items, 2, "the extra probe row must be trimmed from the response")
	assert.True(t, payload.HasMore, "three rows with limit 2 must report a next page")
	assert.Equal(t, 1, payload.Page)
	assert.Equal(t, 2, payload.Limit)
	assert.Contains(t, rec.Body.String(), `"has_more":true`)
}

func TestAdminList_HasMoreFalseOnLastPage(t *testing.T) {
	router, _ := setupRouter(t)
	seedAdminArticles(t, router, 3)

	rec := doRequest(t, router, http.MethodGet, "/api/admin/news?limit=10", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	payload := decodeHasMoreList(t, rec.Body.Bytes())
	require.Len(t, payload.Items, 3)
	assert.False(t, payload.HasMore, "a page holding every row must not report a next page")
	assert.Contains(t, rec.Body.String(), `"has_more":false`)
}

func TestPublicList_HasMoreReflectsNextPage(t *testing.T) {
	router, _ := setupPublicRouter(t)
	createAdminArticle(t, router, "Спорт", "Перша опублікована", "published")
	createAdminArticle(t, router, "Спорт", "Друга опублікована", "published")
	createAdminArticle(t, router, "Спорт", "Третя опублікована", "published")

	rec := doRequest(t, router, http.MethodGet, "/api/news?limit=2", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	payload := decodeHasMoreList(t, rec.Body.Bytes())
	require.Len(t, payload.Items, 2, "the extra probe row must be trimmed from the response")
	assert.True(t, payload.HasMore, "three rows with limit 2 must report a next page")
	assert.Equal(t, 1, payload.Page)
	assert.Equal(t, 2, payload.Limit)
	assert.Contains(t, rec.Body.String(), `"has_more":true`)
}

func TestPublicList_HasMoreFalseOnLastPage(t *testing.T) {
	router, _ := setupPublicRouter(t)
	createAdminArticle(t, router, "Спорт", "Єдина опублікована", "published")

	rec := doRequest(t, router, http.MethodGet, "/api/news?limit=10", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	payload := decodeHasMoreList(t, rec.Body.Bytes())
	require.Len(t, payload.Items, 1)
	assert.False(t, payload.HasMore, "a page holding every row must not report a next page")
	assert.Contains(t, rec.Body.String(), `"has_more":false`)
}

func TestPublicSearch_HasMoreAlwaysFalse(t *testing.T) {
	router, repo := setupPublicRouter(t)
	seedSearchNews(t, repo, "search-first", "Пошукова стаття перша",
		models.CategoryNews, models.StatusPublished, time.Now())
	seedSearchNews(t, repo, "search-second", "Пошукова стаття друга",
		models.CategoryNews, models.StatusPublished, time.Now())

	rec := doRequest(t, router, http.MethodGet, "/api/news/search?q="+url.QueryEscape("пошукова"), "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	payload := decodeHasMoreList(t, rec.Body.Bytes())
	require.Len(t, payload.Items, 2)
	assert.False(t, payload.HasMore, "search is a single suggestion page and never reports a next page")
	assert.Contains(t, rec.Body.String(), `"has_more":false`)
}
