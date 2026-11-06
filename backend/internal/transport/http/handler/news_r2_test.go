package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
	"github.com/maxaixs/mandat/backend/internal/service"
	httptransport "github.com/maxaixs/mandat/backend/internal/transport/http"
	"github.com/maxaixs/mandat/backend/internal/transport/http/handler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
}

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
		ImageUrl:    input.ImageUrl,
		Source:      input.Source,
		SourceURL:   input.SourceURL,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// setupRecordingRouter assembles the admin routes over the recording stub.
func setupRecordingRouter() (http.Handler, *recordingNewsService) {
	stub := &recordingNewsService{}

	httpSrv := httptransport.New(httptransport.HTTPConfig{
		AllowedOrigins: []string{allowedOrigin},
	}, handler.NewNewsHandler(stub))

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

// TestAdminCreate_MultipartCover_PassesBytesToService verifies that a
// multipart create maps every text field and forwards the cover file.
func TestAdminCreate_MultipartCover_PassesBytesToService(t *testing.T) {
	router, stub := setupRecordingRouter()

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
	router, stub := setupRecordingRouter()

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
	router, stub := setupRecordingRouter()

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
	router, stub := setupRecordingRouter()

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
	router, stub := setupRecordingRouter()

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
	router, stub := setupRecordingRouter()
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
	router, stub := setupRecordingRouter()

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
