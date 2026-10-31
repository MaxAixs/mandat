package handler_test

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/maxaixs/mandat/backend/internal/service"
	"github.com/maxaixs/mandat/backend/internal/testsupport"
	httptransport "github.com/maxaixs/mandat/backend/internal/transport/http"
	"github.com/maxaixs/mandat/backend/internal/transport/http/handler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const allowedOrigin = "http://localhost:5173"

func setupRouter() (http.Handler, *testsupport.StubRepo) {
	repo := testsupport.NewStubRepo()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := service.NewNewsService(repo, logger)

	httpSrv := httptransport.New(httptransport.HTTPConfig{
		AllowedOrigins: []string{allowedOrigin},
	}, handler.NewNewsHandler(svc))
	return httpSrv.Handler(), repo
}

// createArticle posts body and returns the slug of the created article.
func createArticle(t *testing.T, router http.Handler, body string) string {
	t.Helper()

	rec := doRequest(t, router, http.MethodPost, "/api/admin/news", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var article map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &article))

	slug, ok := article["slug"].(string)
	require.True(t, ok, "slug must be a string")

	return slug
}

type errorBody struct {
	Error struct {
		Status  int    `json:"status"`
		Message string `json:"message"`
	} `json:"error"`
}

func decodeError(t *testing.T, body []byte) errorBody {
	t.Helper()
	var parsed errorBody
	require.NoError(t, json.Unmarshal(body, &parsed))
	return parsed
}

func doRequest(t *testing.T, router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

const validBody = `{"category":"Новини","title":"Уряд схвалив новий бюджет","text":"Повний текст"}`

const mainEventBody = `{"category":"Новини","title":"Головна подія дня","text":"Повний текст","main_event":true}`

func TestCreate_Returns201WithArticle(t *testing.T) {
	router, repo := setupRouter()

	rec := doRequest(t, router, http.MethodPost, "/api/admin/news", validBody)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.Len(t, repo.Items, 1)

	var article map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &article))
	assert.Equal(t, "draft", article["status"])
	assert.Equal(t, "", article["language"])
	assert.NotEmpty(t, article["id"])
	assert.NotEmpty(t, article["created_at"])
}

func TestCreate_MapsMainEventJSON(t *testing.T) {
	router, repo := setupRouter()

	rec := doRequest(t, router, http.MethodPost, "/api/admin/news", mainEventBody)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var article map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &article))
	assert.Equal(t, true, article["main_event"])
	require.Len(t, repo.Items, 1)
	for _, item := range repo.Items {
		assert.True(t, item.MainEvent, "the request flag must reach the stored article")
	}
}

func TestCreate_InvalidJSONReturns400(t *testing.T) {
	router, _ := setupRouter()

	rec := doRequest(t, router, http.MethodPost, "/api/admin/news", `{"title":`)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "Invalid input", decodeError(t, rec.Body.Bytes()).Error.Message)
}

func TestCreate_MissingTitleReturns400(t *testing.T) {
	router, _ := setupRouter()

	rec := doRequest(t, router, http.MethodPost, "/api/admin/news", `{"category":"Новини"}`)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "Invalid input", decodeError(t, rec.Body.Bytes()).Error.Message)
}

func TestCreate_UnknownCategoryReturns400(t *testing.T) {
	router, _ := setupRouter()

	rec := doRequest(t, router, http.MethodPost, "/api/admin/news", `{"category":"Політика","title":"Тест","text":"Текст"}`)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, `unknown category "Політика"`, decodeError(t, rec.Body.Bytes()).Error.Message)
}

func TestCreate_ConflictReturns409(t *testing.T) {
	router, repo := setupRouter()
	// The slug comes from the title; block every retry suffix to surface the conflict.
	repo.Conflicts["test"] = true
	for attempt := 2; attempt <= 10; attempt++ {
		repo.Conflicts[fmt.Sprintf("test-%d", attempt)] = true
	}

	rec := doRequest(t, router, http.MethodPost, "/api/admin/news", `{"category":"Новини","title":"Тест","text":"Текст"}`)

	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "Resource conflict", decodeError(t, rec.Body.Bytes()).Error.Message)
}

func TestCreate_OversizedBodyReturns413(t *testing.T) {
	router, _ := setupRouter()

	huge := strings.Repeat("a", 2<<20)
	body := `{"category":"Новини","title":"` + huge + `","text":"Текст"}`

	rec := doRequest(t, router, http.MethodPost, "/api/admin/news", body)

	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code, rec.Body.String())
	assert.Equal(t, "Request payload too large", decodeError(t, rec.Body.Bytes()).Error.Message)
}

func TestList_ReturnsEnvelope(t *testing.T) {
	router, _ := setupRouter()
	require.Equal(t, http.StatusCreated, doRequest(t, router, http.MethodPost, "/api/admin/news", validBody).Code)

	rec := doRequest(t, router, http.MethodGet, "/api/admin/news", "")

	require.Equal(t, http.StatusOK, rec.Code)
	var payload struct {
		Items []map[string]any `json:"items"`
		Page  int              `json:"page"`
		Limit int              `json:"limit"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	assert.Len(t, payload.Items, 1)
	assert.Equal(t, 1, payload.Page)
	assert.Equal(t, 20, payload.Limit)
}

func TestList_ReturnsMainEvent(t *testing.T) {
	router, _ := setupRouter()
	createArticle(t, router, mainEventBody)

	rec := doRequest(t, router, http.MethodGet, "/api/admin/news", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var payload struct {
		Items []map[string]any `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Len(t, payload.Items, 1)
	assert.Equal(t, true, payload.Items[0]["main_event"])
}

func TestList_InvalidPaginationReturns400(t *testing.T) {
	router, _ := setupRouter()

	for _, query := range []string{"?page=abc", "?page=0", "?limit=0"} {
		rec := doRequest(t, router, http.MethodGet, "/api/admin/news"+query, "")
		require.Equal(t, http.StatusBadRequest, rec.Code, query)
		assert.Equal(t, "Invalid input", decodeError(t, rec.Body.Bytes()).Error.Message, query)
	}
}

func TestList_ClampsLimit(t *testing.T) {
	router, _ := setupRouter()

	rec := doRequest(t, router, http.MethodGet, "/api/admin/news?limit=1000000", "")

	require.Equal(t, http.StatusOK, rec.Code)
	var payload struct {
		Limit int `json:"limit"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	assert.Equal(t, 100, payload.Limit)
}

func TestGetBySlug_Returns200(t *testing.T) {
	router, _ := setupRouter()
	slug := createArticle(t, router, validBody)

	rec := doRequest(t, router, http.MethodGet, "/api/admin/news/"+slug, "")

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Уряд схвалив новий бюджет")
}

func TestGetBySlug_ReturnsMainEvent(t *testing.T) {
	router, _ := setupRouter()
	slug := createArticle(t, router, mainEventBody)

	rec := doRequest(t, router, http.MethodGet, "/api/admin/news/"+slug, "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var article map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &article))
	assert.Equal(t, true, article["main_event"])
}

func TestGetBySlug_NotFoundReturns404(t *testing.T) {
	router, _ := setupRouter()

	rec := doRequest(t, router, http.MethodGet, "/api/admin/news/missing-article", "")

	require.Equal(t, http.StatusNotFound, rec.Code)
	body := decodeError(t, rec.Body.Bytes())
	assert.Equal(t, http.StatusNotFound, body.Error.Status)
	assert.Equal(t, "News article not found", body.Error.Message)
}

func TestUpdate_Returns200WithChangedFields(t *testing.T) {
	router, _ := setupRouter()
	slug := createArticle(t, router, validBody)

	rec := doRequest(t, router, http.MethodPut, "/api/admin/news/"+slug,
		`{"category":"Міжнародні новини","title":"Оновлений заголовок","text":"Новий текст","featured":true}`)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Оновлений заголовок")
	assert.Contains(t, rec.Body.String(), "Міжнародні новини")
}

func TestUpdate_MapsMainEventJSON(t *testing.T) {
	router, repo := setupRouter()
	slug := createArticle(t, router, validBody)

	rec := doRequest(t, router, http.MethodPut, "/api/admin/news/"+slug, mainEventBody)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var article map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &article))
	assert.Equal(t, true, article["main_event"])
	require.Len(t, repo.Items, 1)
	for _, item := range repo.Items {
		assert.True(t, item.MainEvent, "the request flag must reach the stored article")
	}
}

func TestUpdate_NotFoundReturns404(t *testing.T) {
	router, _ := setupRouter()

	rec := doRequest(t, router, http.MethodPut, "/api/admin/news/missing-article", validBody)

	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "News article not found", decodeError(t, rec.Body.Bytes()).Error.Message)
}

func TestDelete_Returns200(t *testing.T) {
	router, repo := setupRouter()
	slug := createArticle(t, router, validBody)

	rec := doRequest(t, router, http.MethodDelete, "/api/admin/news/"+slug, "")

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, repo.Items)
}

func TestDelete_NotFoundReturns404(t *testing.T) {
	router, _ := setupRouter()

	rec := doRequest(t, router, http.MethodDelete, "/api/admin/news/missing-article", "")

	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "News article not found", decodeError(t, rec.Body.Bytes()).Error.Message)
}

func TestCORS_AllowedOriginIsEchoed(t *testing.T) {
	router, _ := setupRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/admin/news", nil)
	req.Header.Set("Origin", allowedOrigin)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, allowedOrigin, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_PreflightReturns204(t *testing.T) {
	router, _ := setupRouter()

	req := httptest.NewRequest(http.MethodOptions, "/api/admin/news", nil)
	req.Header.Set("Origin", allowedOrigin)
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, allowedOrigin, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_DisallowedOriginHasNoHeader(t *testing.T) {
	router, _ := setupRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/admin/news", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}
