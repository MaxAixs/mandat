package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
	"github.com/maxaixs/mandat/backend/internal/service"
	httptransport "github.com/maxaixs/mandat/backend/internal/transport/http"
	"github.com/maxaixs/mandat/backend/internal/transport/http/handler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubAdFormatService is an AdFormatService double recording the inputs it receives.
type stubAdFormatService struct {
	items       []*models.AdFormat
	listErr     error
	createErr   error
	updateErr   error
	deleteErr   error
	moveErr     error
	createCalls int
	updateCalls int
	deleteCalls int
	moveCalls   int
	lastCreate  service.AdFormatInput
	lastCreateB []byte
	lastUpdate  service.AdFormatInput
	lastUpdateB []byte
	lastUpdateD uuid.UUID
	lastDelete  uuid.UUID
	lastMoveID  uuid.UUID
	lastMoveDir string
}

func (s *stubAdFormatService) List(_ context.Context) ([]*models.AdFormat, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	if s.items == nil {
		return nil, nil
	}
	out := make([]*models.AdFormat, len(s.items))
	copy(out, s.items)

	return out, nil
}

func (s *stubAdFormatService) Create(_ context.Context, input service.AdFormatInput) (*models.AdFormat, error) {
	s.createCalls++
	s.lastCreate = input
	if input.Image != nil && input.Image.Body != nil {
		body, _ := io.ReadAll(input.Image.Body)
		s.lastCreateB = body
	}
	if s.createErr != nil {
		return nil, s.createErr
	}

	return adFormatFromInput(input), nil
}

func (s *stubAdFormatService) Update(
	_ context.Context,
	id uuid.UUID,
	input service.AdFormatInput,
) (*models.AdFormat, error) {
	s.updateCalls++
	s.lastUpdateD = id
	s.lastUpdate = input
	if input.Image != nil && input.Image.Body != nil {
		body, _ := io.ReadAll(input.Image.Body)
		s.lastUpdateB = body
	}
	if s.updateErr != nil {
		return nil, s.updateErr
	}

	format := adFormatFromInput(input)
	format.ID = id

	return format, nil
}

func (s *stubAdFormatService) Delete(_ context.Context, id uuid.UUID) error {
	s.deleteCalls++
	s.lastDelete = id

	return s.deleteErr
}

func (s *stubAdFormatService) Move(_ context.Context, id uuid.UUID, direction string) error {
	s.moveCalls++
	s.lastMoveID = id
	s.lastMoveDir = direction

	return s.moveErr
}

func adFormatFromInput(input service.AdFormatInput) *models.AdFormat {
	return &models.AdFormat{
		ID:          uuid.New(),
		Title:       input.Title,
		Description: input.Description,
		ImageURL:    "https://cdn.example.com/ad-formats/img.png",
		Position:    1,
	}
}

func adFormatFixture(id uuid.UUID, title string, position int) *models.AdFormat {
	return &models.AdFormat{
		ID:          id,
		Title:       title,
		Description: "Опис " + title,
		ImageURL:    "https://cdn.example.com/ad-formats/img.png",
		Position:    position,
	}
}

func setupAdFormatAdminRouter(
	t *testing.T,
	stub *stubAdFormatService,
	middlewares ...gin.HandlerFunc,
) http.Handler {
	t.Helper()

	httpSrv, err := httptransport.New(httptransport.HTTPConfig{
		AllowedOrigins: []string{allowedOrigin},
	}, nil, handler.NewAdFormatHandler(stub, middlewares...))
	require.NoError(t, err)

	return httpSrv.Handler()
}

func setupAdFormatPublicRouter(t *testing.T, stub *stubAdFormatService) http.Handler {
	t.Helper()

	httpSrv, err := httptransport.New(httptransport.HTTPConfig{
		AllowedOrigins: []string{allowedOrigin},
	}, nil, handler.NewPublicAdFormatHandler(stub))
	require.NoError(t, err)

	return httpSrv.Handler()
}

type adFormatListPayload struct {
	Items []*struct {
		ID          uuid.UUID `json:"id"`
		Title       string    `json:"title"`
		Description string    `json:"description"`
		ImageURL    string    `json:"image_url"`
		Position    int       `json:"position"`
	} `json:"items"`
}

func decodeAdFormatList(t *testing.T, body []byte) adFormatListPayload {
	t.Helper()

	var payload adFormatListPayload
	require.NoError(t, json.Unmarshal(body, &payload))

	return payload
}

func TestAdFormatsAdminList_ReturnsItems(t *testing.T) {
	stub := &stubAdFormatService{items: []*models.AdFormat{
		adFormatFixture(uuid.New(), "Перший", 1),
		adFormatFixture(uuid.New(), "Другий", 2),
	}}
	router := setupAdFormatAdminRouter(t, stub)

	rec := doRequest(t, router, http.MethodGet, "/api/admin/ad-formats", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	payload := decodeAdFormatList(t, rec.Body.Bytes())
	require.Len(t, payload.Items, 2)
	assert.Equal(t, "Перший", payload.Items[0].Title)
	assert.Equal(t, "Опис Перший", payload.Items[0].Description)
	assert.Equal(t, "https://cdn.example.com/ad-formats/img.png", payload.Items[0].ImageURL)
	assert.Equal(t, 1, payload.Items[0].Position)
	assert.Equal(t, stub.items[0].ID, payload.Items[0].ID)
}

func TestAdFormatsAdminList_EmptyReturnsEmptyArray(t *testing.T) {
	stub := &stubAdFormatService{}
	router := setupAdFormatAdminRouter(t, stub)

	rec := doRequest(t, router, http.MethodGet, "/api/admin/ad-formats", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"items":[]`,
		"an empty list must serialize as [] rather than null")
	payload := decodeAdFormatList(t, rec.Body.Bytes())
	require.NotNil(t, payload.Items, "an empty list must serialize as [] rather than null")
	assert.Empty(t, payload.Items)
}

func TestAdFormatsAdminCreate_ForwardsImage(t *testing.T) {
	stub := &stubAdFormatService{}
	router := setupAdFormatAdminRouter(t, stub)

	imageBytes := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
	body, contentType := buildMultipartBody(t,
		[]multipartField{
			{name: "title", value: "Новий формат"},
			{name: "description", value: "Опис формату"},
		},
		&multipartFile{
			field:       "image",
			filename:    "a.png",
			contentType: "image/png",
			body:        imageBytes,
		},
	)

	rec := doMultipartRequest(t, router, http.MethodPost, "/api/admin/ad-formats", body, contentType)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	require.Equal(t, 1, stub.createCalls)
	assert.Equal(t, "Новий формат", stub.lastCreate.Title)
	assert.Equal(t, "Опис формату", stub.lastCreate.Description)
	require.NotNil(t, stub.lastCreate.Image, "the image file must reach the service")
	assert.Equal(t, "a.png", stub.lastCreate.Image.Filename)
	require.NotNil(t, stub.lastCreate.Image.Body)
	assert.Equal(t, imageBytes, stub.lastCreateB)

	var card map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &card))
	assert.Equal(t, "Новий формат", card["title"])
	assert.Equal(t, "Опис формату", card["description"])
}

func TestAdFormatsAdminCreate_WithoutImage(t *testing.T) {
	stub := &stubAdFormatService{}
	router := setupAdFormatAdminRouter(t, stub)

	body, contentType := buildMultipartBody(t,
		[]multipartField{
			{name: "title", value: "Без зображення"},
			{name: "description", value: "Опис"},
		},
		nil,
	)

	rec := doMultipartRequest(t, router, http.MethodPost, "/api/admin/ad-formats", body, contentType)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	require.Equal(t, 1, stub.createCalls)
	assert.Nil(t, stub.lastCreate.Image, "a form without a file part must not have an image")
}

func TestAdFormatsAdminCreate_ImageTooLarge(t *testing.T) {
	stub := &stubAdFormatService{}
	router := setupAdFormatAdminRouter(t, stub)

	body, contentType := buildMultipartBody(t,
		[]multipartField{
			{name: "title", value: "Завеликий формат"},
		},
		&multipartFile{
			field:       "image",
			filename:    "big.png",
			contentType: "image/png",
			size:        int(service.MaxCoverBytes) + 1,
		},
	)

	rec := doMultipartRequest(t, router, http.MethodPost, "/api/admin/ad-formats", body, contentType)

	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code, rec.Body.String())
	assert.Equal(t, "Request payload too large", decodeError(t, rec.Body.Bytes()).Error.Message)
	assert.Zero(t, stub.createCalls, "an oversized image must not reach the service")
}

func TestAdFormatsAdminUpdate_RemoveImage(t *testing.T) {
	stub := &stubAdFormatService{}
	router := setupAdFormatAdminRouter(t, stub)
	id := uuid.New()

	body, contentType := buildMultipartBody(t,
		[]multipartField{
			{name: "title", value: "Оновлений"},
			{name: "description", value: "Новий опис"},
			{name: "remove_image", value: "true"},
		},
		nil,
	)

	rec := doMultipartRequest(t, router, http.MethodPut, "/api/admin/ad-formats/"+id.String(), body, contentType)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, 1, stub.updateCalls)
	assert.Equal(t, id, stub.lastUpdateD)
	assert.True(t, stub.lastUpdate.RemoveImage, "remove_image=true must reach the service")
}

func TestAdFormatsAdminUpdate_InvalidRemoveImage(t *testing.T) {
	stub := &stubAdFormatService{}
	router := setupAdFormatAdminRouter(t, stub)

	body, contentType := buildMultipartBody(t,
		[]multipartField{
			{name: "title", value: "Оновлений"},
			{name: "remove_image", value: "nope"},
		},
		nil,
	)

	rec := doMultipartRequest(t, router, http.MethodPut,
		"/api/admin/ad-formats/"+uuid.NewString(), body, contentType)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Zero(t, stub.updateCalls, "an invalid remove_image must not reach the service")
}

func TestAdFormatsAdminUpdate_InvalidUUID(t *testing.T) {
	stub := &stubAdFormatService{}
	router := setupAdFormatAdminRouter(t, stub)

	body, contentType := buildMultipartBody(t,
		[]multipartField{{name: "title", value: "Оновлений"}},
		nil,
	)

	rec := doMultipartRequest(t, router, http.MethodPut, "/api/admin/ad-formats/not-a-uuid", body, contentType)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Zero(t, stub.updateCalls)
}

func TestAdFormatsAdminUpdate_NotFound(t *testing.T) {
	stub := &stubAdFormatService{updateErr: errs.ErrAdFormatNotFound}
	router := setupAdFormatAdminRouter(t, stub)

	body, contentType := buildMultipartBody(t,
		[]multipartField{{name: "title", value: "Оновлений"}},
		nil,
	)

	rec := doMultipartRequest(t, router, http.MethodPut,
		"/api/admin/ad-formats/"+uuid.NewString(), body, contentType)

	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Equal(t, "Ad format not found", decodeError(t, rec.Body.Bytes()).Error.Message)
}

func TestAdFormatsAdminDelete_Returns200(t *testing.T) {
	stub := &stubAdFormatService{}
	router := setupAdFormatAdminRouter(t, stub)
	id := uuid.New()

	rec := doRequest(t, router, http.MethodDelete, "/api/admin/ad-formats/"+id.String(), "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, 1, stub.deleteCalls)
	assert.Equal(t, id, stub.lastDelete)
}

func TestAdFormatsAdminDelete_NotFound(t *testing.T) {
	stub := &stubAdFormatService{deleteErr: errs.ErrAdFormatNotFound}
	router := setupAdFormatAdminRouter(t, stub)

	rec := doRequest(t, router, http.MethodDelete, "/api/admin/ad-formats/"+uuid.NewString(), "")

	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Equal(t, "Ad format not found", decodeError(t, rec.Body.Bytes()).Error.Message)
}

func TestAdFormatsAdminDelete_InvalidUUID(t *testing.T) {
	stub := &stubAdFormatService{}
	router := setupAdFormatAdminRouter(t, stub)

	rec := doRequest(t, router, http.MethodDelete, "/api/admin/ad-formats/not-a-uuid", "")

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Zero(t, stub.deleteCalls)
}

func TestAdFormatsAdminMove_ForwardsDirection(t *testing.T) {
	stub := &stubAdFormatService{}
	router := setupAdFormatAdminRouter(t, stub)
	id := uuid.New()

	rec := doRequest(t, router, http.MethodPost,
		"/api/admin/ad-formats/"+id.String()+"/move", `{"direction":"up"}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, 1, stub.moveCalls)
	assert.Equal(t, id, stub.lastMoveID)
	assert.Equal(t, "up", stub.lastMoveDir)
}

func TestAdFormatsAdminMove_MalformedJSON(t *testing.T) {
	stub := &stubAdFormatService{}
	router := setupAdFormatAdminRouter(t, stub)

	rec := doRequest(t, router, http.MethodPost,
		"/api/admin/ad-formats/"+uuid.NewString()+"/move", `{"direction":`)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Zero(t, stub.moveCalls)
}

func TestAdFormatsAdminMove_InvalidDirection(t *testing.T) {
	stub := &stubAdFormatService{moveErr: errs.ErrInvalidInput}
	router := setupAdFormatAdminRouter(t, stub)

	rec := doRequest(t, router, http.MethodPost,
		"/api/admin/ad-formats/"+uuid.NewString()+"/move", `{"direction":"left"}`)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

func TestAdFormatsAdmin_MiddlewareBlocksAllRoutes(t *testing.T) {
	block := func(c *gin.Context) { c.AbortWithStatus(http.StatusUnauthorized) }
	stub := &stubAdFormatService{items: []*models.AdFormat{adFormatFixture(uuid.New(), "Перший", 1)}}
	router := setupAdFormatAdminRouter(t, stub, block)

	getRec := doRequest(t, router, http.MethodGet, "/api/admin/ad-formats", "")
	require.Equal(t, http.StatusUnauthorized, getRec.Code, "the middleware must guard GET")

	imageBytes := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
	postBody, postType := buildMultipartBody(t,
		[]multipartField{{name: "title", value: "Новий"}},
		&multipartFile{field: "image", filename: "a.png", contentType: "image/png", body: imageBytes},
	)
	postRec := doMultipartRequest(t, router, http.MethodPost, "/api/admin/ad-formats", postBody, postType)
	require.Equal(t, http.StatusUnauthorized, postRec.Code, "the middleware must guard POST")
	assert.Zero(t, stub.createCalls)
}

func TestAdFormatsPublicList_ReturnsItems(t *testing.T) {
	stub := &stubAdFormatService{items: []*models.AdFormat{
		adFormatFixture(uuid.New(), "Перший", 1),
		adFormatFixture(uuid.New(), "Другий", 2),
	}}
	router := setupAdFormatPublicRouter(t, stub)

	rec := doRequest(t, router, http.MethodGet, "/api/ad-formats", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	payload := decodeAdFormatList(t, rec.Body.Bytes())
	require.Len(t, payload.Items, 2)
	assert.Equal(t, "Перший", payload.Items[0].Title)
	assert.Equal(t, 1, payload.Items[0].Position)
	assert.Equal(t, "https://cdn.example.com/ad-formats/img.png", payload.Items[0].ImageURL)
}

func TestAdFormatsPublicList_ServiceError500(t *testing.T) {
	stub := &stubAdFormatService{listErr: errors.New("db is down")}
	router := setupAdFormatPublicRouter(t, stub)

	rec := doRequest(t, router, http.MethodGet, "/api/ad-formats", "")

	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
	body := decodeError(t, rec.Body.Bytes())
	assert.Equal(t, http.StatusInternalServerError, body.Error.Status)
	assert.Equal(t, "Internal server error", body.Error.Message)
	assert.NotContains(t, rec.Body.String(), "db is down",
		"the response must not leak internal details")
}

func TestAdFormatsAdminMove_InvalidUUID(t *testing.T) {
	stub := &stubAdFormatService{}
	router := setupAdFormatAdminRouter(t, stub)

	rec := doRequest(t, router, http.MethodPost,
		"/api/admin/ad-formats/not-a-uuid/move", `{"direction":"up"}`)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Zero(t, stub.moveCalls)
}

func TestAdFormatsAdminMove_NotFound(t *testing.T) {
	stub := &stubAdFormatService{moveErr: errs.ErrAdFormatNotFound}
	router := setupAdFormatAdminRouter(t, stub)

	rec := doRequest(t, router, http.MethodPost,
		"/api/admin/ad-formats/"+uuid.NewString()+"/move", `{"direction":"up"}`)

	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Equal(t, "Ad format not found", decodeError(t, rec.Body.Bytes()).Error.Message)
}

func TestAdFormatsAdminCreate_ValidationError(t *testing.T) {
	stub := &stubAdFormatService{createErr: errs.NewValidationError("title", "title is required")}
	router := setupAdFormatAdminRouter(t, stub)

	body, contentType := buildMultipartBody(t,
		[]multipartField{{name: "title", value: ""}},
		nil,
	)

	rec := doMultipartRequest(t, router, http.MethodPost, "/api/admin/ad-formats", body, contentType)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Equal(t, "title is required", decodeError(t, rec.Body.Bytes()).Error.Message)
}

func TestAdFormatsAdminUpdate_ValidationError(t *testing.T) {
	stub := &stubAdFormatService{updateErr: errs.NewValidationError("title", "title is required")}
	router := setupAdFormatAdminRouter(t, stub)

	body, contentType := buildMultipartBody(t,
		[]multipartField{{name: "title", value: ""}},
		nil,
	)

	rec := doMultipartRequest(t, router, http.MethodPut,
		"/api/admin/ad-formats/"+uuid.NewString(), body, contentType)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Equal(t, "title is required", decodeError(t, rec.Body.Bytes()).Error.Message)
}

func TestAdFormatsAdminList_ServiceError500(t *testing.T) {
	stub := &stubAdFormatService{listErr: errors.New("db is down")}
	router := setupAdFormatAdminRouter(t, stub)

	rec := doRequest(t, router, http.MethodGet, "/api/admin/ad-formats", "")

	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
	body := decodeError(t, rec.Body.Bytes())
	assert.Equal(t, http.StatusInternalServerError, body.Error.Status)
	assert.Equal(t, "Internal server error", body.Error.Message)
	assert.NotContains(t, rec.Body.String(), "db is down",
		"the response must not leak internal details")
}
