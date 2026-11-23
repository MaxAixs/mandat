package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
	"github.com/maxaixs/mandat/backend/internal/service"
	"github.com/maxaixs/mandat/backend/internal/testsupport"
	httptransport "github.com/maxaixs/mandat/backend/internal/transport/http"
	"github.com/maxaixs/mandat/backend/internal/transport/http/dto"
	"github.com/maxaixs/mandat/backend/internal/transport/http/handler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Shared fakes and helpers: stub NewsService, router setup, request/decoding helpers ---
const allowedOrigin = "http://localhost:5173"

func setupRouter(t *testing.T) (http.Handler, *testsupport.StubRepo) {
	t.Helper()
	repo := testsupport.NewStubRepo()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := service.NewNewsService(repo, logger, nil, "", nil)

	httpSrv, err := httptransport.New(httptransport.HTTPConfig{
		AllowedOrigins: []string{allowedOrigin},
	}, nil, handler.NewNewsHandler(svc))
	require.NoError(t, err)
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

// r2ArticleSlug is the slug the recording stub returns for every write.
const r2ArticleSlug = "multipart-article"

// recordingNewsService is a NewsService double that records the inputs it
// receives, including the raw bytes of an uploaded cover.
type recordingNewsService struct {
	creates     []service.NewsInput
	updates     []service.NewsInput
	createCalls int
	updateCalls int
	coverBodies [][]byte
	createErr   error

	videoCalls           int
	videoLastContentType string
	videoLastSize        int64
	videoUpload          *service.PresignedUpload
	videoErr             error

	imageCalls           int
	imageLastContentType string
	imageLastSize        int64
	imageUpload          *service.PresignedUpload
	imageErr             error
}

var _ handler.NewsService = (*recordingNewsService)(nil)

// Create records the input and returns createErr when it is set, which lets a
// test mimic the service-side validation that now owns required fields.
func (s *recordingNewsService) Create(_ context.Context, input service.NewsInput) (*models.News, error) {
	s.createCalls++
	s.creates = append(s.creates, input)

	if s.createErr != nil {
		return nil, s.createErr
	}

	s.recordCover(input)

	return r2Article(input), nil
}

// Update records the input and returns a deterministic article.
func (s *recordingNewsService) Update(_ context.Context, slug string, input service.NewsInput) (*models.News, error) {
	s.updateCalls++
	s.updates = append(s.updates, input)
	s.recordCover(input)

	return r2Article(input), nil
}

// GetBySlug is not exercised by these tests.
func (s *recordingNewsService) GetBySlug(_ context.Context, slug string) (*models.News, error) {
	news := r2Article(service.NewsInput{})
	news.Slug = slug

	return news, nil
}

// List is not exercised by these tests.
func (s *recordingNewsService) List(
	_ context.Context,
	page int,
	limit int,
	status string,
	category string,
) (*service.ListResult, error) {
	return &service.ListResult{Page: page, Limit: limit}, nil
}

// Delete is not exercised by these tests.
func (s *recordingNewsService) Delete(_ context.Context, slug string) error {
	return nil
}

// PresignVideoUpload records the request and returns the scripted result.
func (s *recordingNewsService) PresignVideoUpload(
	_ context.Context,
	req service.UploadRequest,
) (*service.PresignedUpload, error) {
	s.videoCalls++
	s.videoLastContentType = req.ContentType
	s.videoLastSize = req.Size
	if s.videoErr != nil {
		return nil, s.videoErr
	}

	return s.videoUpload, nil
}

// PresignImageUpload records the request and returns the scripted result.
func (s *recordingNewsService) PresignImageUpload(
	_ context.Context,
	req service.UploadRequest,
) (*service.PresignedUpload, error) {
	s.imageCalls++
	s.imageLastContentType = req.ContentType
	s.imageLastSize = req.Size
	if s.imageErr != nil {
		return nil, s.imageErr
	}

	return s.imageUpload, nil
}

// recordCover drains and stores the cover body, if any.
func (s *recordingNewsService) recordCover(input service.NewsInput) {
	if input.Cover == nil || input.Cover.Body == nil {
		s.coverBodies = append(s.coverBodies, nil)

		return
	}

	body, _ := io.ReadAll(input.Cover.Body)
	s.coverBodies = append(s.coverBodies, body)
}

// r2Article builds a minimal article for the response payload.
func r2Article(input service.NewsInput) *models.News {
	now := time.Now().UTC()

	return &models.News{
		ID:          uuid.New(),
		Slug:        r2ArticleSlug,
		Category:    input.Category,
		Title:       input.Title,
		Description: input.Description,
		Text:        input.Text,
		Language:    input.Language,
		Status:      models.StatusDraft,
		Featured:    input.Featured,
		MainEvent:   input.MainEvent,
		ImageURL:    input.ImageURL,
		Source:      input.Source,
		SourceURL:   input.SourceURL,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// setupRecordingRouter assembles the admin routes over the recording stub.
func setupRecordingRouter(t *testing.T) (http.Handler, *recordingNewsService) {
	t.Helper()
	stub := &recordingNewsService{}

	httpSrv, err := httptransport.New(httptransport.HTTPConfig{
		AllowedOrigins: []string{allowedOrigin},
	}, nil, handler.NewNewsHandler(stub))
	require.NoError(t, err)

	return httpSrv.Handler(), stub
}

// multipartField is one text field of a multipart form.
type multipartField struct {
	name  string
	value string
}

// multipartFile is one file part of a multipart form. When size is positive
// the part is filled with size repeated bytes instead of body.
type multipartFile struct {
	field       string
	filename    string
	contentType string
	body        []byte
	size        int
}

// buildMultipartBody builds a multipart body and its Content-Type header.
func buildMultipartBody(
	t *testing.T,
	fields []multipartField,
	file *multipartFile,
) ([]byte, string) {
	t.Helper()

	var buf bytes.Buffer

	writer := multipart.NewWriter(&buf)
	for _, field := range fields {
		require.NoError(t, writer.WriteField(field.name, field.value))
	}

	if file != nil {
		header := textproto.MIMEHeader{}
		header.Set(
			"Content-Disposition",
			fmt.Sprintf(`form-data; name=%q; filename=%q`, file.field, file.filename),
		)
		header.Set("Content-Type", file.contentType)

		part, err := writer.CreatePart(header)
		require.NoError(t, err)

		if file.size > 0 {
			require.NoError(t, writeRepeated(part, file.size))
		} else {
			_, err = part.Write(file.body)
			require.NoError(t, err)
		}
	}

	require.NoError(t, writer.Close())

	return buf.Bytes(), writer.FormDataContentType()
}

// writeRepeated writes size bytes of "a" without allocating one large slice.
func writeRepeated(w io.Writer, size int) error {
	const chunkSize = 64 << 10

	chunk := bytes.Repeat([]byte("a"), chunkSize)
	for written := 0; written < size; written += chunkSize {
		n := chunkSize
		if remaining := size - written; remaining < n {
			n = remaining
		}
		if _, err := w.Write(chunk[:n]); err != nil {
			return err
		}
	}

	return nil
}

// doMultipartRequest sends body as a multipart request.
func doMultipartRequest(
	t *testing.T,
	router http.Handler,
	method string,
	path string,
	body []byte,
	contentType string,
) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", contentType)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	return rec
}

// stubNewsVideoService is a NewsService double recording the presign inputs.
// It implements the unified handler.NewsService: CRUD methods are inert,
// the two presign methods record the request and return scripted results.
type stubNewsVideoService struct {
	calls           int
	lastContentType string
	lastSize        int64
	upload          *service.PresignedUpload
	err             error

	imageCalls           int
	imageLastContentType string
	imageLastSize        int64
	imageUpload          *service.PresignedUpload
	imageErr             error
}

var _ handler.NewsService = (*stubNewsVideoService)(nil)

func (s *stubNewsVideoService) Create(
	_ context.Context,
	input service.NewsInput,
) (*models.News, error) {
	return r2Article(input), nil
}

func (s *stubNewsVideoService) GetBySlug(_ context.Context, slug string) (*models.News, error) {
	news := r2Article(service.NewsInput{})
	news.Slug = slug

	return news, nil
}

func (s *stubNewsVideoService) List(
	_ context.Context,
	page int,
	limit int,
	_ string,
	_ string,
) (*service.ListResult, error) {
	return &service.ListResult{Page: page, Limit: limit}, nil
}

func (s *stubNewsVideoService) Update(
	_ context.Context,
	slug string,
	input service.NewsInput,
) (*models.News, error) {
	news := r2Article(input)
	news.Slug = slug

	return news, nil
}

func (s *stubNewsVideoService) Delete(_ context.Context, _ string) error {
	return nil
}

func (s *stubNewsVideoService) PresignVideoUpload(
	_ context.Context,
	req service.UploadRequest,
) (*service.PresignedUpload, error) {
	s.calls++
	s.lastContentType = req.ContentType
	s.lastSize = req.Size
	if s.err != nil {
		return nil, s.err
	}

	return s.upload, nil
}

func (s *stubNewsVideoService) PresignImageUpload(
	_ context.Context,
	req service.UploadRequest,
) (*service.PresignedUpload, error) {
	s.imageCalls++
	s.imageLastContentType = req.ContentType
	s.imageLastSize = req.Size
	if s.imageErr != nil {
		return nil, s.imageErr
	}

	return s.imageUpload, nil
}

// setupNewsVideoRouter assembles the unified news routes over the stub.
func setupNewsVideoRouter(
	t *testing.T,
	stub *stubNewsVideoService,
	middlewares ...gin.HandlerFunc,
) http.Handler {
	t.Helper()

	httpSrv, err := httptransport.New(httptransport.HTTPConfig{
		AllowedOrigins: []string{allowedOrigin},
	}, nil, handler.NewNewsHandler(stub, middlewares...))
	require.NoError(t, err)

	return httpSrv.Handler()
}

const videoPendingKey = "pending/videos/0123456789abcdef0123456789abcdef.mp4"

// stubNewsImageService is a NewsService double recording the presign inputs.
// It implements the unified handler.NewsService: CRUD methods are inert,
// the two presign methods record the request and return scripted results.
type stubNewsImageService struct {
	calls           int
	lastContentType string
	lastSize        int64
	upload          *service.PresignedUpload
	err             error

	videoCalls           int
	videoLastContentType string
	videoLastSize        int64
	videoUpload          *service.PresignedUpload
	videoErr             error
}

var _ handler.NewsService = (*stubNewsImageService)(nil)

func (s *stubNewsImageService) Create(
	_ context.Context,
	input service.NewsInput,
) (*models.News, error) {
	return r2Article(input), nil
}

func (s *stubNewsImageService) GetBySlug(_ context.Context, slug string) (*models.News, error) {
	news := r2Article(service.NewsInput{})
	news.Slug = slug

	return news, nil
}

func (s *stubNewsImageService) List(
	_ context.Context,
	page int,
	limit int,
	_ string,
	_ string,
) (*service.ListResult, error) {
	return &service.ListResult{Page: page, Limit: limit}, nil
}

func (s *stubNewsImageService) Update(
	_ context.Context,
	slug string,
	input service.NewsInput,
) (*models.News, error) {
	news := r2Article(input)
	news.Slug = slug

	return news, nil
}

func (s *stubNewsImageService) Delete(_ context.Context, _ string) error {
	return nil
}

func (s *stubNewsImageService) PresignImageUpload(
	_ context.Context,
	req service.UploadRequest,
) (*service.PresignedUpload, error) {
	s.calls++
	s.lastContentType = req.ContentType
	s.lastSize = req.Size
	if s.err != nil {
		return nil, s.err
	}

	return s.upload, nil
}

func (s *stubNewsImageService) PresignVideoUpload(
	_ context.Context,
	req service.UploadRequest,
) (*service.PresignedUpload, error) {
	s.videoCalls++
	s.videoLastContentType = req.ContentType
	s.videoLastSize = req.Size
	if s.videoErr != nil {
		return nil, s.videoErr
	}

	return s.videoUpload, nil
}

// setupNewsImageRouter assembles the unified news routes over the stub.
func setupNewsImageRouter(
	t *testing.T,
	stub *stubNewsImageService,
	middlewares ...gin.HandlerFunc,
) http.Handler {
	t.Helper()

	httpSrv, err := httptransport.New(httptransport.HTTPConfig{
		AllowedOrigins: []string{allowedOrigin},
	}, nil, handler.NewNewsHandler(stub, middlewares...))
	require.NoError(t, err)

	return httpSrv.Handler()
}

const imagePendingKey = "pending/images/0123456789abcdef0123456789abcdef.jpg"

// --- Admin news CRUD: create/list/get/update/delete over JSON ---
func TestCreate_Returns201WithArticle(t *testing.T) {
	router, repo := setupRouter(t)

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
	router, repo := setupRouter(t)

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
	router, _ := setupRouter(t)

	rec := doRequest(t, router, http.MethodPost, "/api/admin/news", `{"title":`)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "Invalid input", decodeError(t, rec.Body.Bytes()).Error.Message)
}

func TestCreate_MissingTitleReturns400(t *testing.T) {
	router, _ := setupRouter(t)

	rec := doRequest(t, router, http.MethodPost, "/api/admin/news", `{"category":"Новини"}`)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "Invalid input", decodeError(t, rec.Body.Bytes()).Error.Message)
}

func TestCreate_UnknownCategoryReturns400(t *testing.T) {
	router, _ := setupRouter(t)

	rec := doRequest(t, router, http.MethodPost, "/api/admin/news", `{"category":"Фантастика","title":"Тест","text":"Текст"}`)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, `unknown category "Фантастика"`, decodeError(t, rec.Body.Bytes()).Error.Message)
}

func TestCreate_AcceptsNewCategories(t *testing.T) {
	tests := []struct {
		name     string
		category models.Category
	}{
		{"economics", models.CategoryEconomics},
		{"politics", models.CategoryPolitics},
		{"regional", models.CategoryRegional},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, repo := setupRouter(t)

			body := fmt.Sprintf(`{"category":%q,"title":"Тестова стаття","text":"Повний текст"}`, tt.category)
			rec := doRequest(t, router, http.MethodPost, "/api/admin/news", body)

			require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
			require.Len(t, repo.Items, 1)
			for _, item := range repo.Items {
				assert.Equal(t, tt.category, item.Category,
					"the stored article must keep the requested rubric")
			}
		})
	}
}

func TestCreate_ConflictReturns409(t *testing.T) {
	router, repo := setupRouter(t)
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
	router, _ := setupRouter(t)

	huge := strings.Repeat("a", 2<<20)
	body := `{"category":"Новини","title":"` + huge + `","text":"Текст"}`

	rec := doRequest(t, router, http.MethodPost, "/api/admin/news", body)

	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code, rec.Body.String())
	assert.Equal(t, "Request payload too large", decodeError(t, rec.Body.Bytes()).Error.Message)
}

func TestList_ReturnsEnvelope(t *testing.T) {
	router, _ := setupRouter(t)
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
	router, _ := setupRouter(t)
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
	router, _ := setupRouter(t)

	for _, query := range []string{"?page=abc", "?page=0", "?limit=0"} {
		rec := doRequest(t, router, http.MethodGet, "/api/admin/news"+query, "")
		require.Equal(t, http.StatusBadRequest, rec.Code, query)
		assert.Equal(t, "Invalid input", decodeError(t, rec.Body.Bytes()).Error.Message, query)
	}
}

func TestList_ClampsLimit(t *testing.T) {
	router, _ := setupRouter(t)

	rec := doRequest(t, router, http.MethodGet, "/api/admin/news?limit=1000000", "")

	require.Equal(t, http.StatusOK, rec.Code)
	var payload struct {
		Limit int `json:"limit"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	assert.Equal(t, 100, payload.Limit)
}

func TestGetBySlug_Returns200(t *testing.T) {
	router, _ := setupRouter(t)
	slug := createArticle(t, router, validBody)

	rec := doRequest(t, router, http.MethodGet, "/api/admin/news/"+slug, "")

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Уряд схвалив новий бюджет")
}

func TestGetBySlug_ReturnsMainEvent(t *testing.T) {
	router, _ := setupRouter(t)
	slug := createArticle(t, router, mainEventBody)

	rec := doRequest(t, router, http.MethodGet, "/api/admin/news/"+slug, "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var article map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &article))
	assert.Equal(t, true, article["main_event"])
}

func TestGetBySlug_NotFoundReturns404(t *testing.T) {
	router, _ := setupRouter(t)

	rec := doRequest(t, router, http.MethodGet, "/api/admin/news/missing-article", "")

	require.Equal(t, http.StatusNotFound, rec.Code)
	body := decodeError(t, rec.Body.Bytes())
	assert.Equal(t, http.StatusNotFound, body.Error.Status)
	assert.Equal(t, "News article not found", body.Error.Message)
}

func TestUpdate_Returns200WithChangedFields(t *testing.T) {
	router, _ := setupRouter(t)
	slug := createArticle(t, router, validBody)

	rec := doRequest(t, router, http.MethodPut, "/api/admin/news/"+slug,
		`{"category":"Міжнародні новини","title":"Оновлений заголовок","text":"Новий текст","featured":true}`)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Оновлений заголовок")
	assert.Contains(t, rec.Body.String(), "Міжнародні новини")
}

func TestUpdate_MapsMainEventJSON(t *testing.T) {
	router, repo := setupRouter(t)
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
	router, _ := setupRouter(t)

	rec := doRequest(t, router, http.MethodPut, "/api/admin/news/missing-article", validBody)

	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "News article not found", decodeError(t, rec.Body.Bytes()).Error.Message)
}

func TestDelete_Returns200(t *testing.T) {
	router, repo := setupRouter(t)
	slug := createArticle(t, router, validBody)

	rec := doRequest(t, router, http.MethodDelete, "/api/admin/news/"+slug, "")

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, repo.Items)
}

func TestDelete_NotFoundReturns404(t *testing.T) {
	router, _ := setupRouter(t)

	rec := doRequest(t, router, http.MethodDelete, "/api/admin/news/missing-article", "")

	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "News article not found", decodeError(t, rec.Body.Bytes()).Error.Message)
}

func TestCORS_AllowedOriginIsEchoed(t *testing.T) {
	router, _ := setupRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/news", nil)
	req.Header.Set("Origin", allowedOrigin)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, allowedOrigin, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_PreflightReturns204(t *testing.T) {
	router, _ := setupRouter(t)

	req := httptest.NewRequest(http.MethodOptions, "/api/admin/news", nil)
	req.Header.Set("Origin", allowedOrigin)
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, allowedOrigin, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_DisallowedOriginHasNoHeader(t *testing.T) {
	router, _ := setupRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/news", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

// --- Pagination: has_more on admin lists ---
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

// --- Cover uploads: multipart cover file handling and size limits ---
// TestAdminCreate_MultipartCover_PassesBytesToService verifies that a
// multipart create maps every text field and forwards the cover file.
func TestAdminCreate_MultipartCover_PassesBytesToService(t *testing.T) {
	router, stub := setupRecordingRouter(t)

	coverBytes := []byte("fake-jpeg-bytes")
	body, contentType := buildMultipartBody(t,
		[]multipartField{
			{name: "category", value: "Новини"},
			{name: "title", value: "Мультипарт стаття"},
			{name: "text", value: "Повний текст"},
			{name: "language", value: "uk"},
			{name: "featured", value: "true"},
		},
		&multipartFile{
			field:       "cover",
			filename:    "cover.jpg",
			contentType: "image/jpeg",
			body:        coverBytes,
		},
	)

	rec := doMultipartRequest(t, router, http.MethodPost, "/api/admin/news", body, contentType)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	require.Len(t, stub.creates, 1)

	input := stub.creates[0]
	require.NotNil(t, input.Cover, "the cover file must reach the service")
	assert.Equal(t, "cover.jpg", input.Cover.Filename)
	assert.Equal(t, "image/jpeg", input.Cover.ContentType)
	assert.Equal(t, int64(len(coverBytes)), input.Cover.Size)
	require.Len(t, stub.coverBodies, 1)
	assert.Equal(t, coverBytes, stub.coverBodies[0],
		"the opened cover body must be forwarded unchanged")

	assert.Equal(t, models.Category("Новини"), input.Category)
	assert.Equal(t, "Мультипарт стаття", input.Title)
	assert.Equal(t, "Повний текст", input.Text)
	assert.Equal(t, "uk", input.Language)
	assert.True(t, input.Featured, "the featured form field must map onto the input")
}

// TestAdminCreate_JSONWithoutCover_StillWorks verifies the existing JSON path
// is untouched and does not invent a cover.
func TestAdminCreate_JSONWithoutCover_StillWorks(t *testing.T) {
	router, stub := setupRecordingRouter(t)

	rec := doRequest(t, router, http.MethodPost, "/api/admin/news",
		`{"category":"Новини","title":"JSON стаття","text":"Текст"}`)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	require.Len(t, stub.creates, 1)

	input := stub.creates[0]
	assert.Nil(t, input.Cover, "a JSON request must not produce a cover upload")
	assert.Equal(t, models.Category("Новини"), input.Category)
	assert.Equal(t, "JSON стаття", input.Title)
	assert.Equal(t, "Текст", input.Text)

	var article map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &article))
	assert.Equal(t, "JSON стаття", article["title"])
}

// TestAdminUpdate_MultipartCover_PassesBytesToService verifies that a
// multipart update forwards the cover file to the service.
func TestAdminUpdate_MultipartCover_PassesBytesToService(t *testing.T) {
	router, stub := setupRecordingRouter(t)

	coverBytes := []byte("fake-png-bytes")
	body, contentType := buildMultipartBody(t,
		[]multipartField{
			{name: "category", value: "Міжнародні новини"},
			{name: "title", value: "Оновлений заголовок"},
			{name: "text", value: "Новий текст"},
		},
		&multipartFile{
			field:       "cover",
			filename:    "new-cover.png",
			contentType: "image/png",
			body:        coverBytes,
		},
	)

	rec := doMultipartRequest(t, router, http.MethodPut,
		"/api/admin/news/"+r2ArticleSlug, body, contentType)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Len(t, stub.updates, 1)

	input := stub.updates[0]
	require.NotNil(t, input.Cover, "the cover file must reach the service")
	assert.Equal(t, "new-cover.png", input.Cover.Filename)
	assert.Equal(t, "image/png", input.Cover.ContentType)
	assert.Equal(t, int64(len(coverBytes)), input.Cover.Size)
	require.Len(t, stub.coverBodies, 1)
	assert.Equal(t, coverBytes, stub.coverBodies[0],
		"the opened cover body must be forwarded unchanged")

	assert.Equal(t, models.Category("Міжнародні новини"), input.Category)
	assert.Equal(t, "Оновлений заголовок", input.Title)
	assert.Equal(t, "Новий текст", input.Text)
}

// TestAdminCreate_InvalidBoolField_Rejected verifies a non-boolean form flag
// is a client error rather than being silently treated as false.
func TestAdminCreate_InvalidBoolField_Rejected(t *testing.T) {
	router, stub := setupRecordingRouter(t)

	body, contentType := buildMultipartBody(t,
		[]multipartField{
			{name: "category", value: "Новини"},
			{name: "title", value: "Некоректний прапорець"},
			{name: "featured", value: "yes"},
		},
		nil,
	)

	rec := doMultipartRequest(t, router, http.MethodPost, "/api/admin/news", body, contentType)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Equal(t, "Invalid input", decodeError(t, rec.Body.Bytes()).Error.Message)
	assert.Zero(t, stub.createCalls, "an invalid boolean must not reach the service")
}

// TestAdminCreate_MultipartWithoutCover_Works verifies a multipart create
// without a file part is still valid and does not invent a cover.
func TestAdminCreate_MultipartWithoutCover_Works(t *testing.T) {
	router, stub := setupRecordingRouter(t)

	body, contentType := buildMultipartBody(t,
		[]multipartField{
			{name: "category", value: "Новини"},
			{name: "title", value: "Без обкладинки"},
			{name: "text", value: "Повний текст"},
			{name: "language", value: "uk"},
		},
		nil,
	)

	rec := doMultipartRequest(t, router, http.MethodPost, "/api/admin/news", body, contentType)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	require.Len(t, stub.creates, 1)

	input := stub.creates[0]
	assert.Nil(t, input.Cover, "a multipart form without a file part must not have a cover")
	assert.Equal(t, "Новини", string(input.Category))
	assert.Equal(t, "Без обкладинки", input.Title)
	assert.Equal(t, "Повний текст", input.Text)
	assert.Equal(t, "uk", input.Language)
}

// TestAdminCreate_MultipartMissingTitle_Rejected verifies the required fields
// are enforced for multipart requests too. Validation lives in the service, so
// the request reaches it and its error is mapped centrally onto the envelope.
func TestAdminCreate_MultipartMissingTitle_Rejected(t *testing.T) {
	router, stub := setupRecordingRouter(t)
	// The real service rejects an empty title with this field-scoped error.
	stub.createErr = errs.NewValidationError("title", "title is required")

	body, contentType := buildMultipartBody(t,
		[]multipartField{
			{name: "category", value: "Новини"},
			{name: "text", value: "Повний текст"},
		},
		nil,
	)

	rec := doMultipartRequest(t, router, http.MethodPost, "/api/admin/news", body, contentType)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Equal(t, 1, stub.createCalls,
		"validation happens in the service, so the request must reach it")
	require.Len(t, stub.creates, 1)
	assert.Empty(t, stub.creates[0].Title, "the transport must forward the missing title unchanged")

	errBody := decodeError(t, rec.Body.Bytes())
	assert.Equal(t, http.StatusBadRequest, errBody.Error.Status)
	assert.Equal(t, "title is required", errBody.Error.Message)
}

// TestAdminCreate_MultipartTooLarge_Rejected verifies an oversized multipart
// body is rejected as a payload error before reaching the service.
func TestAdminCreate_MultipartTooLarge_Rejected(t *testing.T) {
	router, stub := setupRecordingRouter(t)

	body, contentType := buildMultipartBody(t,
		[]multipartField{
			{name: "category", value: "Новини"},
			{name: "title", value: "Завелика стаття"},
			{name: "text", value: "Повний текст"},
		},
		&multipartFile{
			field:       "cover",
			filename:    "huge.jpg",
			contentType: "image/jpeg",
			size:        12 << 20,
		},
	)

	rec := doMultipartRequest(t, router, http.MethodPost, "/api/admin/news", body, contentType)

	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code, rec.Body.String())
	assert.Equal(t, "Request payload too large", decodeError(t, rec.Body.Bytes()).Error.Message)
	assert.Zero(t, stub.createCalls, "an oversized body must not reach the service")
	assert.Empty(t, stub.creates)
}

// --- Video: presign endpoint /video-uploads and video fields in requests/responses ---
func TestNewsVideoUpload_PresignSuccess_ReturnsShapeAndForwardsInput(t *testing.T) {
	expiresAt := time.Now().UTC().Truncate(time.Second).Add(15 * time.Minute)
	stub := &stubNewsVideoService{upload: &service.PresignedUpload{
		Key:       videoPendingKey,
		UploadURL: "https://r2.example.com/put",
		ExpiresAt: expiresAt,
	}}
	router := setupNewsVideoRouter(t, stub)

	rec := doRequest(t, router, http.MethodPost, "/api/admin/news/video-uploads",
		`{"content_type":"video/mp4","size":1048576}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, 1, stub.calls)
	assert.Equal(t, "video/mp4", stub.lastContentType, "the service must receive the exact content type")
	assert.Equal(t, int64(1048576), stub.lastSize, "the service must receive the exact size")

	var payload map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	assert.Equal(t, videoPendingKey, payload["key"])
	assert.Equal(t, "https://r2.example.com/put", payload["upload_url"])
	rawExpires, ok := payload["expires_at"].(string)
	require.True(t, ok, "expires_at must be a string, got %v", payload["expires_at"])
	parsed, err := time.Parse(time.RFC3339Nano, rawExpires)
	require.NoError(t, err, "expires_at must be RFC3339")
	assert.WithinDuration(t, expiresAt, parsed, time.Second)
}

func TestNewsVideoUpload_PresignMalformedJSON_Returns400(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"truncated", `{"content_type":`},
		{"wrong type", `{"content_type":42,"size":"huge"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &stubNewsVideoService{upload: &service.PresignedUpload{}}
			router := setupNewsVideoRouter(t, stub)

			rec := doRequest(t, router, http.MethodPost, "/api/admin/news/video-uploads", tt.body)

			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			assert.Equal(t, "Invalid input", decodeError(t, rec.Body.Bytes()).Error.Message)
			assert.Zero(t, stub.calls, "malformed input must not reach the service")
		})
	}
}

func TestNewsVideoUpload_PresignValidationError_Returns400(t *testing.T) {
	stub := &stubNewsVideoService{err: errs.NewValidationError("content_type", "video must be mp4 or webm")}
	router := setupNewsVideoRouter(t, stub)

	rec := doRequest(t, router, http.MethodPost, "/api/admin/news/video-uploads",
		`{"content_type":"video/avi","size":10}`)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Equal(t, "video must be mp4 or webm", decodeError(t, rec.Body.Bytes()).Error.Message)
	assert.Equal(t, 1, stub.calls)
}

func TestNewsVideoUpload_PresignServiceUnavailable_Returns503(t *testing.T) {
	stub := &stubNewsVideoService{err: errs.ErrServiceUnavailable}
	router := setupNewsVideoRouter(t, stub)

	rec := doRequest(t, router, http.MethodPost, "/api/admin/news/video-uploads",
		`{"content_type":"video/mp4","size":10}`)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
	assert.Equal(t, "Service unavailable", decodeError(t, rec.Body.Bytes()).Error.Message)
}

func TestNewsVideoUpload_MiddlewareBlocks_Returns401(t *testing.T) {
	block := func(c *gin.Context) { c.AbortWithStatus(http.StatusUnauthorized) }
	stub := &stubNewsVideoService{upload: &service.PresignedUpload{}}
	router := setupNewsVideoRouter(t, stub, block)

	rec := doRequest(t, router, http.MethodPost, "/api/admin/news/video-uploads",
		`{"content_type":"video/mp4","size":10}`)

	require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
	assert.Zero(t, stub.calls, "an aborted request must not reach the service")
}

func TestNewsCreate_JSONForwardsVideoFields(t *testing.T) {
	router, stub := setupRecordingRouter(t)

	rec := doRequest(t, router, http.MethodPost, "/api/admin/news",
		`{"category":"Новини","title":"Відео стаття","text":"Текст",`+
			`"video_key":"`+videoPendingKey+`","video_url":"https://youtu.be/dQw4w9WgXcQ"}`)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	require.Len(t, stub.creates, 1)
	assert.Equal(t, videoPendingKey, stub.creates[0].VideoKey)
	assert.Equal(t, "https://youtu.be/dQw4w9WgXcQ", stub.creates[0].VideoURL)
}

func TestNewsUpdate_JSONForwardsVideoFields(t *testing.T) {
	router, stub := setupRecordingRouter(t)

	rec := doRequest(t, router, http.MethodPut, "/api/admin/news/"+r2ArticleSlug,
		`{"category":"Новини","title":"Оновлена","text":"Текст",`+
			`"video_key":"`+videoPendingKey+`","video_url":"https://vimeo.com/123456789"}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Len(t, stub.updates, 1)
	assert.Equal(t, videoPendingKey, stub.updates[0].VideoKey)
	assert.Equal(t, "https://vimeo.com/123456789", stub.updates[0].VideoURL)
}

func TestNewsCreate_MultipartForwardsVideoFields(t *testing.T) {
	router, stub := setupRecordingRouter(t)

	body, contentType := buildMultipartBody(t,
		[]multipartField{
			{name: "category", value: "Новини"},
			{name: "title", value: "Мультипарт відео"},
			{name: "text", value: "Повний текст"},
			{name: "video_key", value: videoPendingKey},
			{name: "video_url", value: "https://youtu.be/dQw4w9WgXcQ"},
		},
		nil,
	)

	rec := doMultipartRequest(t, router, http.MethodPost, "/api/admin/news", body, contentType)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	require.Len(t, stub.creates, 1)
	assert.Equal(t, videoPendingKey, stub.creates[0].VideoKey)
	assert.Equal(t, "https://youtu.be/dQw4w9WgXcQ", stub.creates[0].VideoURL)
}

func TestNewsUpdate_MultipartForwardsVideoFields(t *testing.T) {
	router, stub := setupRecordingRouter(t)

	body, contentType := buildMultipartBody(t,
		[]multipartField{
			{name: "category", value: "Новини"},
			{name: "title", value: "Оновлена"},
			{name: "text", value: "Новий текст"},
			{name: "video_key", value: videoPendingKey},
			{name: "video_url", value: "https://vimeo.com/123456789"},
		},
		nil,
	)

	rec := doMultipartRequest(t, router, http.MethodPut,
		"/api/admin/news/"+r2ArticleSlug, body, contentType)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Len(t, stub.updates, 1)
	assert.Equal(t, videoPendingKey, stub.updates[0].VideoKey)
	assert.Equal(t, "https://vimeo.com/123456789", stub.updates[0].VideoURL)
}

func TestNewsRequest_ToInput_MapsVideoFields(t *testing.T) {
	req := dto.NewsRequest{
		Category: "Новини",
		Title:    "Відео",
		VideoKey: videoPendingKey,
		VideoURL: "https://youtu.be/dQw4w9WgXcQ",
	}

	input := req.ToInput()

	assert.Equal(t, videoPendingKey, input.VideoKey)
	assert.Equal(t, "https://youtu.be/dQw4w9WgXcQ", input.VideoURL)
}

func TestNewsToResponse_VideoShapes(t *testing.T) {
	tests := []struct {
		name string
		kind models.VideoKind
		url  string
	}{
		{"file", models.VideoKindFile, "https://cdn.example.com/videos/a.mp4"},
		{"embed", models.VideoKindEmbed, "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			news := &models.News{
				Category: models.CategoryNews,
				Title:    "Відео",
				Video:    &models.Video{Kind: tt.kind, URL: tt.url},
			}

			resp := dto.NewsToResponse(news)

			require.NotNil(t, resp.Video)
			assert.Equal(t, string(tt.kind), resp.Video.Kind)
			assert.Equal(t, tt.url, resp.Video.URL)
		})
	}
}

func TestNewsToResponse_NilVideoIsNull(t *testing.T) {
	resp := dto.NewsToResponse(&models.News{Category: models.CategoryNews, Title: "Без відео"})

	raw, err := json.Marshal(resp)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.Contains(t, decoded, "video", "a missing video must serialize as null, not be omitted")
	assert.Nil(t, decoded["video"])
}

func TestNewsToPublicDetailResponse_VideoShapes(t *testing.T) {
	tests := []struct {
		name string
		kind models.VideoKind
		url  string
	}{
		{"file", models.VideoKindFile, "https://cdn.example.com/videos/a.mp4"},
		{"embed", models.VideoKindEmbed, "https://player.vimeo.com/video/123456789"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			news := &models.News{
				Slug:     "video-article",
				Category: models.CategoryNews,
				Title:    "Відео",
				Video:    &models.Video{Kind: tt.kind, URL: tt.url},
			}

			resp := dto.NewsToPublicDetailResponse(news)

			require.NotNil(t, resp.Video)
			assert.Equal(t, string(tt.kind), resp.Video.Kind)
			assert.Equal(t, tt.url, resp.Video.URL)
		})
	}
}

func TestNewsToPublicDetailResponse_NilVideoIsNull(t *testing.T) {
	resp := dto.NewsToPublicDetailResponse(&models.News{Slug: "plain", Title: "Без відео"})

	raw, err := json.Marshal(resp)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.Contains(t, decoded, "video", "a missing video must serialize as null, not be omitted")
	assert.Nil(t, decoded["video"])
}

func TestNewsToPublicResponse_OmitsVideoKey(t *testing.T) {
	resp := dto.NewsToPublicResponse(&models.News{
		Slug:     "video-article",
		Category: models.CategoryNews,
		Title:    "Відео",
		Video:    &models.Video{Kind: models.VideoKindFile, URL: "https://cdn.example.com/videos/a.mp4"},
	})

	raw, err := json.Marshal(resp)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), `"video"`, "public list items must not expose a video key")
}

func TestAdminGet_VideoIsExposed(t *testing.T) {
	router, repo := setupRouter(t)
	slug := createArticle(t, router, validBody)
	for _, item := range repo.Items {
		item.Video = &models.Video{Kind: models.VideoKindFile, URL: "https://cdn.example.com/videos/a.mp4"}
	}

	rec := doRequest(t, router, http.MethodGet, "/api/admin/news/"+slug, "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var article map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &article))
	video, ok := article["video"].(map[string]any)
	require.True(t, ok, "admin detail must contain a video object, got %v", article["video"])
	assert.Equal(t, "file", video["kind"])
	assert.Equal(t, "https://cdn.example.com/videos/a.mp4", video["url"])
}

func TestAdminCreate_VideoEmbedIsExposed(t *testing.T) {
	router, _ := setupRouter(t)

	rec := doRequest(t, router, http.MethodPost, "/api/admin/news",
		`{"category":"Новини","title":"Вбудоване відео","text":"Текст",`+
			`"video_url":"https://youtu.be/dQw4w9WgXcQ"}`)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var article map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &article))
	video, ok := article["video"].(map[string]any)
	require.True(t, ok, "admin create must echo the embed video, got %v", article["video"])
	assert.Equal(t, "embed", video["kind"])
	assert.Equal(t, "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ", video["url"])
}

func TestPublicDetail_VideoIsExposed(t *testing.T) {
	router, repo := setupPublicRouter(t)
	slug := createArticle(t, router, publishedBody)
	for _, item := range repo.Items {
		if item.Slug == slug {
			item.Video = &models.Video{
				Kind: models.VideoKindEmbed,
				URL:  "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ",
			}
		}
	}

	rec := doRequest(t, router, http.MethodGet, "/api/news/"+slug, "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	detail := decodePublicDetail(t, rec.Body.Bytes())
	video, ok := detail["video"].(map[string]any)
	require.True(t, ok, "public detail must contain a video object, got %v", detail["video"])
	assert.Equal(t, "embed", video["kind"])
	assert.Equal(t, "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ", video["url"])
}

func TestPublicList_VideoIsAbsent(t *testing.T) {
	router, repo := setupPublicRouter(t)
	slug := createArticle(t, router, publishedBody)
	for _, item := range repo.Items {
		if item.Slug == slug {
			item.Video = &models.Video{Kind: models.VideoKindFile, URL: "https://cdn.example.com/videos/a.mp4"}
		}
	}

	rec := doRequest(t, router, http.MethodGet, "/api/news", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	payload := decodePublicList(t, rec.Body.Bytes())
	require.Len(t, payload.Items, 1)
	assert.NotContains(t, payload.Items[0], "video", "public list items must not expose a video key")
}

// --- Images: presign endpoint /image-uploads and images in requests/responses ---
func TestNewsImageUpload_PresignSuccess_ReturnsShapeAndForwardsInput(t *testing.T) {
	expiresAt := time.Now().UTC().Truncate(time.Second).Add(15 * time.Minute)
	stub := &stubNewsImageService{upload: &service.PresignedUpload{
		Key:       imagePendingKey,
		UploadURL: "https://r2.example.com/put",
		ExpiresAt: expiresAt,
	}}
	router := setupNewsImageRouter(t, stub)

	rec := doRequest(t, router, http.MethodPost, "/api/admin/news/image-uploads",
		`{"content_type":"image/jpeg","size":1048576}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, 1, stub.calls)
	assert.Equal(t, "image/jpeg", stub.lastContentType, "the service must receive the exact content type")
	assert.Equal(t, int64(1048576), stub.lastSize, "the service must receive the exact size")

	var payload map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	assert.Equal(t, imagePendingKey, payload["key"])
	assert.Equal(t, "https://r2.example.com/put", payload["upload_url"])
	rawExpires, ok := payload["expires_at"].(string)
	require.True(t, ok, "expires_at must be a string, got %v", payload["expires_at"])
	parsed, err := time.Parse(time.RFC3339Nano, rawExpires)
	require.NoError(t, err, "expires_at must be RFC3339")
	assert.WithinDuration(t, expiresAt, parsed, time.Second)
}

func TestNewsImageUpload_PresignMalformedJSON_Returns400(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"truncated", `{"content_type":`},
		{"wrong type", `{"content_type":42,"size":"huge"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &stubNewsImageService{upload: &service.PresignedUpload{}}
			router := setupNewsImageRouter(t, stub)

			rec := doRequest(t, router, http.MethodPost, "/api/admin/news/image-uploads", tt.body)

			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			assert.Equal(t, "Invalid input", decodeError(t, rec.Body.Bytes()).Error.Message)
			assert.Zero(t, stub.calls, "malformed input must not reach the service")
		})
	}
}

func TestNewsImageUpload_PresignValidationError_Returns400(t *testing.T) {
	stub := &stubNewsImageService{err: errs.NewValidationError("content_type", "image must be jpg, png, webp or gif")}
	router := setupNewsImageRouter(t, stub)

	rec := doRequest(t, router, http.MethodPost, "/api/admin/news/image-uploads",
		`{"content_type":"image/tiff","size":10}`)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Equal(t, "image must be jpg, png, webp or gif", decodeError(t, rec.Body.Bytes()).Error.Message)
	assert.Equal(t, 1, stub.calls)
}

func TestNewsImageUpload_PresignServiceUnavailable_Returns503(t *testing.T) {
	stub := &stubNewsImageService{err: errs.ErrServiceUnavailable}
	router := setupNewsImageRouter(t, stub)

	rec := doRequest(t, router, http.MethodPost, "/api/admin/news/image-uploads",
		`{"content_type":"image/jpeg","size":10}`)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
	assert.Equal(t, "Service unavailable", decodeError(t, rec.Body.Bytes()).Error.Message)
}

func TestNewsImageUpload_MiddlewareBlocks_Returns401(t *testing.T) {
	block := func(c *gin.Context) { c.AbortWithStatus(http.StatusUnauthorized) }
	stub := &stubNewsImageService{upload: &service.PresignedUpload{}}
	router := setupNewsImageRouter(t, stub, block)

	rec := doRequest(t, router, http.MethodPost, "/api/admin/news/image-uploads",
		`{"content_type":"image/jpeg","size":10}`)

	require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
	assert.Zero(t, stub.calls, "an aborted request must not reach the service")
}

func TestNewsCreate_JSONForwardsImages(t *testing.T) {
	router, stub := setupRecordingRouter(t)

	rec := doRequest(t, router, http.MethodPost, "/api/admin/news",
		`{"category":"Новини","title":"Стаття з фото","text":"Текст",`+
			`"images":["`+imagePendingKey+`","https://example.com/extra.jpg"]}`)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	require.Len(t, stub.creates, 1)
	assert.Equal(t, []string{imagePendingKey, "https://example.com/extra.jpg"}, stub.creates[0].Images)
}

func TestNewsUpdate_JSONForwardsImages(t *testing.T) {
	router, stub := setupRecordingRouter(t)

	rec := doRequest(t, router, http.MethodPut, "/api/admin/news/"+r2ArticleSlug,
		`{"category":"Новини","title":"Оновлена","text":"Текст",`+
			`"images":["https://example.com/kept.jpg","`+imagePendingKey+`"]}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Len(t, stub.updates, 1)
	assert.Equal(t, []string{"https://example.com/kept.jpg", imagePendingKey}, stub.updates[0].Images)
}

func TestNewsCreate_MultipartForwardsImagesInOrder(t *testing.T) {
	router, stub := setupRecordingRouter(t)

	body, contentType := buildMultipartBody(t,
		[]multipartField{
			{name: "category", value: "Новини"},
			{name: "title", value: "Мультипарт фото"},
			{name: "text", value: "Повний текст"},
			{name: "images", value: "https://example.com/first.jpg"},
			{name: "images", value: imagePendingKey},
			{name: "images", value: "https://example.com/third.jpg"},
		},
		nil,
	)

	rec := doMultipartRequest(t, router, http.MethodPost, "/api/admin/news", body, contentType)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	require.Len(t, stub.creates, 1)
	assert.Equal(t,
		[]string{"https://example.com/first.jpg", imagePendingKey, "https://example.com/third.jpg"},
		stub.creates[0].Images,
		"repeated images fields must reach the service in order")
}

func TestNewsUpdate_MultipartForwardsImagesInOrder(t *testing.T) {
	router, stub := setupRecordingRouter(t)

	body, contentType := buildMultipartBody(t,
		[]multipartField{
			{name: "category", value: "Новини"},
			{name: "title", value: "Оновлена"},
			{name: "text", value: "Новий текст"},
			{name: "images", value: imagePendingKey},
			{name: "images", value: "https://example.com/second.jpg"},
		},
		nil,
	)

	rec := doMultipartRequest(t, router, http.MethodPut,
		"/api/admin/news/"+r2ArticleSlug, body, contentType)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Len(t, stub.updates, 1)
	assert.Equal(t,
		[]string{imagePendingKey, "https://example.com/second.jpg"},
		stub.updates[0].Images,
		"repeated images fields must reach the service in order")
}

func TestNewsCreate_MultipartWithoutImages_KeepsNil(t *testing.T) {
	router, stub := setupRecordingRouter(t)

	body, contentType := buildMultipartBody(t,
		[]multipartField{
			{name: "category", value: "Новини"},
			{name: "title", value: "Без фото"},
			{name: "text", value: "Повний текст"},
		},
		nil,
	)

	rec := doMultipartRequest(t, router, http.MethodPost, "/api/admin/news", body, contentType)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	require.Len(t, stub.creates, 1)
	assert.Nil(t, stub.creates[0].Images, "a form without images fields must leave images untouched")
}

func TestNewsRequest_ToInput_MapsImagesInOrder(t *testing.T) {
	images := []string{imagePendingKey, "https://example.com/kept.jpg", "https://example.com/new.jpg"}
	req := dto.NewsRequest{
		Category: "Новини",
		Title:    "Фото",
		Images:   images,
	}

	input := req.ToInput()

	assert.Equal(t, images, input.Images, "image order is the display order and must survive")
}

func TestNewsRequest_ToInput_PreservesImagesNilVersusEmpty(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantNil bool
	}{
		{"missing key", `{"category":"Новини","title":"Без фото"}`, true},
		{"empty array", `{"category":"Новини","title":"Без фото","images":[]}`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req dto.NewsRequest
			require.NoError(t, json.Unmarshal([]byte(tt.body), &req))

			input := req.ToInput()

			if tt.wantNil {
				assert.Nil(t, input.Images, "a missing images key must keep the current images")
			} else {
				require.NotNil(t, input.Images, "an explicit empty list must replace the images")
				assert.Empty(t, input.Images)
			}
		})
	}
}

func TestNewsToResponse_ImagesDefaultToEmptyArray(t *testing.T) {
	tests := []struct {
		name   string
		images []string
	}{
		{"nil", nil},
		{"empty", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := dto.NewsToResponse(&models.News{
				Category:  models.CategoryNews,
				Title:     "Без фото",
				ImageURLs: tt.images,
			})

			require.NotNil(t, resp.Images, "admin images must never be null")
			assert.Empty(t, resp.Images)

			raw, err := json.Marshal(resp)
			require.NoError(t, err)
			assert.Contains(t, string(raw), `"images":[]`, "admin images must serialize as an empty array")
		})
	}
}

func TestNewsToResponse_ImagesPreserveOrder(t *testing.T) {
	images := []string{
		"https://cdn.example.com/images/b-first.jpg",
		"https://cdn.example.com/images/a-second.jpg",
	}

	resp := dto.NewsToResponse(&models.News{
		Category:  models.CategoryNews,
		Title:     "З фото",
		ImageURLs: images,
	})

	assert.Equal(t, images, resp.Images, "image order is the display order and must survive")

	raw, err := json.Marshal(resp)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"images":["https://cdn.example.com/images/b-first.jpg","https://cdn.example.com/images/a-second.jpg"]`)
}

func TestNewsToPublicDetailResponse_ImagesDefaultToEmptyArray(t *testing.T) {
	tests := []struct {
		name   string
		images []string
	}{
		{"nil", nil},
		{"empty", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := dto.NewsToPublicDetailResponse(&models.News{
				Slug:      "plain",
				Title:     "Без фото",
				ImageURLs: tt.images,
			})

			require.NotNil(t, resp.Images, "public detail images must never be null")
			assert.Empty(t, resp.Images)

			raw, err := json.Marshal(resp)
			require.NoError(t, err)
			assert.Contains(t, string(raw), `"images":[]`, "public detail images must serialize as an empty array")
		})
	}
}

func TestNewsToPublicDetailResponse_ImagesPreserveOrder(t *testing.T) {
	images := []string{
		"https://cdn.example.com/images/b-first.jpg",
		"https://cdn.example.com/images/a-second.jpg",
	}

	resp := dto.NewsToPublicDetailResponse(&models.News{
		Slug:      "image-article",
		Category:  models.CategoryNews,
		Title:     "З фото",
		ImageURLs: images,
	})

	assert.Equal(t, images, resp.Images, "image order is the display order and must survive")
}

func TestNewsToPublicResponse_OmitsImages(t *testing.T) {
	resp := dto.NewsToPublicResponse(&models.News{
		Slug:      "image-article",
		Category:  models.CategoryNews,
		Title:     "З фото",
		ImageURLs: []string{"https://cdn.example.com/images/a.jpg"},
	})

	raw, err := json.Marshal(resp)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), `"images"`, "public list items must not expose a images key")
}

func TestAdminGet_ImagesAreExposed(t *testing.T) {
	router, repo := setupRouter(t)
	slug := createArticle(t, router, validBody)
	want := []string{"https://cdn.example.com/images/a.jpg", "https://cdn.example.com/images/b.jpg"}
	for _, item := range repo.Items {
		item.ImageURLs = want
	}

	rec := doRequest(t, router, http.MethodGet, "/api/admin/news/"+slug, "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var article map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &article))
	got, ok := article["images"].([]any)
	require.True(t, ok, "admin detail must contain a images array, got %v", article["images"])
	assert.Equal(t, []any{want[0], want[1]}, got)
}

func TestAdminGet_ImagesDefaultToEmptyArray(t *testing.T) {
	router, _ := setupRouter(t)
	slug := createArticle(t, router, validBody)

	rec := doRequest(t, router, http.MethodGet, "/api/admin/news/"+slug, "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var article map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &article))
	images, ok := article["images"].([]any)
	require.True(t, ok, "admin detail without images must expose an array, got %v", article["images"])
	assert.Empty(t, images)
	assert.Contains(t, rec.Body.String(), `"images":[]`)
}

func TestPublicDetail_ImagesAreExposed(t *testing.T) {
	router, repo := setupPublicRouter(t)
	slug := createArticle(t, router, publishedBody)
	want := []string{"https://cdn.example.com/images/a.jpg", "https://cdn.example.com/images/b.jpg"}
	for _, item := range repo.Items {
		if item.Slug == slug {
			item.ImageURLs = want
		}
	}

	rec := doRequest(t, router, http.MethodGet, "/api/news/"+slug, "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	detail := decodePublicDetail(t, rec.Body.Bytes())
	got, ok := detail["images"].([]any)
	require.True(t, ok, "public detail must contain a images array, got %v", detail["images"])
	assert.Equal(t, []any{want[0], want[1]}, got)
}

func TestPublicDetail_ImagesDefaultToEmptyArray(t *testing.T) {
	router, _ := setupPublicRouter(t)
	slug := createArticle(t, router, publishedBody)

	rec := doRequest(t, router, http.MethodGet, "/api/news/"+slug, "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	detail := decodePublicDetail(t, rec.Body.Bytes())
	images, ok := detail["images"].([]any)
	require.True(t, ok, "public detail without images must expose an array, got %v", detail["images"])
	assert.Empty(t, images)
	assert.Contains(t, rec.Body.String(), `"images":[]`)
}

func TestPublicList_ImagesAreAbsent(t *testing.T) {
	router, repo := setupPublicRouter(t)
	slug := createArticle(t, router, publishedBody)
	for _, item := range repo.Items {
		if item.Slug == slug {
			item.ImageURLs = []string{"https://cdn.example.com/images/a.jpg"}
		}
	}

	rec := doRequest(t, router, http.MethodGet, "/api/news", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	payload := decodePublicList(t, rec.Body.Bytes())
	require.Len(t, payload.Items, 1)
	assert.NotContains(t, payload.Items[0], "images", "public list items must not expose a images key")
}
