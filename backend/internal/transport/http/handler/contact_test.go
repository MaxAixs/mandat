package handler_test

import (
	"context"
	"encoding/json"
	"errors"
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

// stubContactService is a ContactService double recording the inputs it receives.
type stubContactService struct {
	items       []*models.Contact
	listErr     error
	createErr   error
	updateErr   error
	deleteErr   error
	moveErr     error
	createCalls int
	updateCalls int
	deleteCalls int
	moveCalls   int
	lastCreate  service.ContactInput
	lastUpdate  service.ContactInput
	lastUpdateD uuid.UUID
	lastDelete  uuid.UUID
	lastMoveID  uuid.UUID
	lastMoveDir string
}

func (s *stubContactService) List(_ context.Context) ([]*models.Contact, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	if s.items == nil {
		return nil, nil
	}
	out := make([]*models.Contact, len(s.items))
	copy(out, s.items)

	return out, nil
}

func (s *stubContactService) Create(_ context.Context, input service.ContactInput) (*models.Contact, error) {
	s.createCalls++
	s.lastCreate = input
	if s.createErr != nil {
		return nil, s.createErr
	}

	return contactFromInput(input), nil
}

func (s *stubContactService) Update(
	_ context.Context,
	id uuid.UUID,
	input service.ContactInput,
) (*models.Contact, error) {
	s.updateCalls++
	s.lastUpdateD = id
	s.lastUpdate = input
	if s.updateErr != nil {
		return nil, s.updateErr
	}

	contact := contactFromInput(input)
	contact.ID = id

	return contact, nil
}

func (s *stubContactService) Delete(_ context.Context, id uuid.UUID) error {
	s.deleteCalls++
	s.lastDelete = id

	return s.deleteErr
}

func (s *stubContactService) Move(_ context.Context, id uuid.UUID, direction string) error {
	s.moveCalls++
	s.lastMoveID = id
	s.lastMoveDir = direction

	return s.moveErr
}

func contactFromInput(input service.ContactInput) *models.Contact {
	return &models.Contact{
		ID:       uuid.New(),
		Type:     models.ContactType(input.Type),
		Value:    input.Value,
		Label:    input.Label,
		Display:  input.Value,
		Href:     "https://example.com/" + input.Value,
		Position: 1,
	}
}

func contactFixture(id uuid.UUID, contactType, value, label string, position int) *models.Contact {
	return &models.Contact{
		ID:       id,
		Type:     models.ContactType(contactType),
		Value:    value,
		Label:    label,
		Display:  value,
		Href:     "https://example.com/" + value,
		Position: position,
	}
}

func setupContactAdminRouter(
	t *testing.T,
	stub *stubContactService,
	middlewares ...gin.HandlerFunc,
) http.Handler {
	t.Helper()

	httpSrv, err := httptransport.New(httptransport.HTTPConfig{
		AllowedOrigins: []string{allowedOrigin},
	}, nil, handler.NewContactHandler(stub, middlewares...))
	require.NoError(t, err)

	return httpSrv.Handler()
}

func setupContactPublicRouter(t *testing.T, stub *stubContactService) http.Handler {
	t.Helper()

	httpSrv, err := httptransport.New(httptransport.HTTPConfig{
		AllowedOrigins: []string{allowedOrigin},
	}, nil, handler.NewContactHandler(stub))
	require.NoError(t, err)

	return httpSrv.Handler()
}

type contactListPayload struct {
	Items []*struct {
		ID       uuid.UUID `json:"id"`
		Type     string    `json:"type"`
		Value    string    `json:"value"`
		Label    string    `json:"label"`
		Display  string    `json:"display"`
		Href     string    `json:"href"`
		Position int       `json:"position"`
	} `json:"items"`
}

func decodeContactList(t *testing.T, body []byte) contactListPayload {
	t.Helper()

	var payload contactListPayload
	require.NoError(t, json.Unmarshal(body, &payload))

	return payload
}

func TestContactsAdminList_ReturnsItems(t *testing.T) {
	stub := &stubContactService{items: []*models.Contact{
		contactFixture(uuid.New(), "email", "info@mandat.ua", "Редакція", 1),
		contactFixture(uuid.New(), "telegram", "mandat", "", 2),
	}}
	router := setupContactAdminRouter(t, stub)

	rec := doRequest(t, router, http.MethodGet, "/api/admin/contacts", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	payload := decodeContactList(t, rec.Body.Bytes())
	require.Len(t, payload.Items, 2)
	assert.Equal(t, stub.items[0].ID, payload.Items[0].ID)
	assert.Equal(t, "email", payload.Items[0].Type)
	assert.Equal(t, "info@mandat.ua", payload.Items[0].Value)
	assert.Equal(t, "Редакція", payload.Items[0].Label)
	assert.Equal(t, "info@mandat.ua", payload.Items[0].Display)
	assert.Equal(t, "https://example.com/info@mandat.ua", payload.Items[0].Href)
	assert.Equal(t, 1, payload.Items[0].Position)
	assert.Equal(t, "telegram", payload.Items[1].Type)
	assert.Equal(t, 2, payload.Items[1].Position)
}

func TestContactsAdminList_EmptyReturnsEmptyArray(t *testing.T) {
	stub := &stubContactService{}
	router := setupContactAdminRouter(t, stub)

	rec := doRequest(t, router, http.MethodGet, "/api/admin/contacts", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"items":[]`,
		"an empty list must serialize as [] rather than null")
	payload := decodeContactList(t, rec.Body.Bytes())
	require.NotNil(t, payload.Items, "an empty list must serialize as [] rather than null")
	assert.Empty(t, payload.Items)
}

func TestContactsPublicList_ReturnsItems(t *testing.T) {
	stub := &stubContactService{items: []*models.Contact{
		contactFixture(uuid.New(), "email", "info@mandat.ua", "Редакція", 1),
		contactFixture(uuid.New(), "telegram", "mandat", "", 2),
	}}
	router := setupContactPublicRouter(t, stub)

	rec := doRequest(t, router, http.MethodGet, "/api/contacts", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	payload := decodeContactList(t, rec.Body.Bytes())
	require.Len(t, payload.Items, 2)
	assert.Equal(t, stub.items[0].ID, payload.Items[0].ID)
	assert.Equal(t, "email", payload.Items[0].Type)
	assert.Equal(t, "info@mandat.ua", payload.Items[0].Value)
	assert.Equal(t, "Редакція", payload.Items[0].Label)
	assert.Equal(t, "info@mandat.ua", payload.Items[0].Display)
	assert.Equal(t, "https://example.com/info@mandat.ua", payload.Items[0].Href)
	assert.Equal(t, 1, payload.Items[0].Position)
}

func TestContactsPublicList_EmptyReturnsEmptyArray(t *testing.T) {
	stub := &stubContactService{}
	router := setupContactPublicRouter(t, stub)

	rec := doRequest(t, router, http.MethodGet, "/api/contacts", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"items":[]`,
		"an empty list must serialize as [] rather than null")
	payload := decodeContactList(t, rec.Body.Bytes())
	require.NotNil(t, payload.Items, "an empty list must serialize as [] rather than null")
	assert.Empty(t, payload.Items)
}

func TestContactsAdminList_ServiceError500(t *testing.T) {
	stub := &stubContactService{listErr: errors.New("db is down")}
	router := setupContactAdminRouter(t, stub)

	rec := doRequest(t, router, http.MethodGet, "/api/admin/contacts", "")

	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
	body := decodeError(t, rec.Body.Bytes())
	assert.Equal(t, http.StatusInternalServerError, body.Error.Status)
	assert.Equal(t, "Internal server error", body.Error.Message)
	assert.NotContains(t, rec.Body.String(), "db is down",
		"the response must not leak internal details")
}

func TestContactsPublicList_ServiceError500(t *testing.T) {
	stub := &stubContactService{listErr: errors.New("db is down")}
	router := setupContactPublicRouter(t, stub)

	rec := doRequest(t, router, http.MethodGet, "/api/contacts", "")

	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
	body := decodeError(t, rec.Body.Bytes())
	assert.Equal(t, http.StatusInternalServerError, body.Error.Status)
	assert.Equal(t, "Internal server error", body.Error.Message)
	assert.NotContains(t, rec.Body.String(), "db is down",
		"the response must not leak internal details")
}

func TestContactsAdminCreate_ForwardsInput(t *testing.T) {
	stub := &stubContactService{}
	router := setupContactAdminRouter(t, stub)

	rec := doRequest(t, router, http.MethodPost, "/api/admin/contacts",
		`{"type":"email","value":"info@mandat.ua","label":"Редакція"}`)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	require.Equal(t, 1, stub.createCalls)
	assert.Equal(t, service.ContactInput{Type: "email", Value: "info@mandat.ua", Label: "Редакція"},
		stub.lastCreate, "the JSON body must reach the service exactly")

	var item map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &item))
	assert.Equal(t, "email", item["type"])
	assert.Equal(t, "info@mandat.ua", item["value"])
	assert.Equal(t, "Редакція", item["label"])
	assert.NotEmpty(t, item["id"])
	assert.NotEmpty(t, item["display"])
	assert.NotEmpty(t, item["href"])
}

func TestContactsAdminCreate_MalformedJSON(t *testing.T) {
	stub := &stubContactService{}
	router := setupContactAdminRouter(t, stub)

	rec := doRequest(t, router, http.MethodPost, "/api/admin/contacts", `{"type":`)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Zero(t, stub.createCalls)
}

func TestContactsAdminCreate_ValidationError(t *testing.T) {
	stub := &stubContactService{createErr: errs.NewValidationError("value", "value is required")}
	router := setupContactAdminRouter(t, stub)

	rec := doRequest(t, router, http.MethodPost, "/api/admin/contacts",
		`{"type":"email","value":"not-an-email"}`)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Equal(t, "value is required", decodeError(t, rec.Body.Bytes()).Error.Message)
}

func TestContactsAdminUpdate_Returns200(t *testing.T) {
	stub := &stubContactService{}
	router := setupContactAdminRouter(t, stub)
	id := uuid.New()

	rec := doRequest(t, router, http.MethodPut, "/api/admin/contacts/"+id.String(),
		`{"type":"telegram","value":"mandat","label":""}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, 1, stub.updateCalls)
	assert.Equal(t, id, stub.lastUpdateD)
	assert.Equal(t, service.ContactInput{Type: "telegram", Value: "mandat"},
		stub.lastUpdate, "the JSON body must reach the service exactly")

	var item map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &item))
	assert.Equal(t, "telegram", item["type"])
	assert.Equal(t, "mandat", item["value"])
}

func TestContactsAdminUpdate_MalformedJSON(t *testing.T) {
	stub := &stubContactService{}
	router := setupContactAdminRouter(t, stub)

	rec := doRequest(t, router, http.MethodPut,
		"/api/admin/contacts/"+uuid.NewString(), `{"type":`)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Zero(t, stub.updateCalls)
}

func TestContactsAdminUpdate_InvalidUUID(t *testing.T) {
	stub := &stubContactService{}
	router := setupContactAdminRouter(t, stub)

	rec := doRequest(t, router, http.MethodPut, "/api/admin/contacts/not-a-uuid",
		`{"type":"email","value":"info@mandat.ua"}`)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Zero(t, stub.updateCalls)
}

func TestContactsAdminUpdate_NotFound(t *testing.T) {
	stub := &stubContactService{updateErr: errs.ErrContactNotFound}
	router := setupContactAdminRouter(t, stub)

	rec := doRequest(t, router, http.MethodPut, "/api/admin/contacts/"+uuid.NewString(),
		`{"type":"email","value":"info@mandat.ua"}`)

	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Equal(t, "Contact not found", decodeError(t, rec.Body.Bytes()).Error.Message)
}

func TestContactsAdminUpdate_ValidationError(t *testing.T) {
	stub := &stubContactService{updateErr: errs.NewValidationError("value", "value is required")}
	router := setupContactAdminRouter(t, stub)

	rec := doRequest(t, router, http.MethodPut, "/api/admin/contacts/"+uuid.NewString(),
		`{"type":"email","value":"not-an-email"}`)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Equal(t, "value is required", decodeError(t, rec.Body.Bytes()).Error.Message)
}

func TestContactsAdminDelete_Returns200(t *testing.T) {
	stub := &stubContactService{}
	router := setupContactAdminRouter(t, stub)
	id := uuid.New()

	rec := doRequest(t, router, http.MethodDelete, "/api/admin/contacts/"+id.String(), "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, 1, stub.deleteCalls)
	assert.Equal(t, id, stub.lastDelete)
}

func TestContactsAdminDelete_NotFound(t *testing.T) {
	stub := &stubContactService{deleteErr: errs.ErrContactNotFound}
	router := setupContactAdminRouter(t, stub)

	rec := doRequest(t, router, http.MethodDelete, "/api/admin/contacts/"+uuid.NewString(), "")

	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Equal(t, "Contact not found", decodeError(t, rec.Body.Bytes()).Error.Message)
}

func TestContactsAdminDelete_InvalidUUID(t *testing.T) {
	stub := &stubContactService{}
	router := setupContactAdminRouter(t, stub)

	rec := doRequest(t, router, http.MethodDelete, "/api/admin/contacts/not-a-uuid", "")

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Zero(t, stub.deleteCalls)
}

func TestContactsAdminMove_ForwardsDirection(t *testing.T) {
	stub := &stubContactService{}
	router := setupContactAdminRouter(t, stub)
	id := uuid.New()

	rec := doRequest(t, router, http.MethodPost,
		"/api/admin/contacts/"+id.String()+"/move", `{"direction":"down"}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, 1, stub.moveCalls)
	assert.Equal(t, id, stub.lastMoveID)
	assert.Equal(t, "down", stub.lastMoveDir)
}

func TestContactsAdminMove_MalformedJSON(t *testing.T) {
	stub := &stubContactService{}
	router := setupContactAdminRouter(t, stub)

	rec := doRequest(t, router, http.MethodPost,
		"/api/admin/contacts/"+uuid.NewString()+"/move", `{"direction":`)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Zero(t, stub.moveCalls)
}

func TestContactsAdminMove_InvalidUUID(t *testing.T) {
	stub := &stubContactService{}
	router := setupContactAdminRouter(t, stub)

	rec := doRequest(t, router, http.MethodPost,
		"/api/admin/contacts/not-a-uuid/move", `{"direction":"up"}`)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Zero(t, stub.moveCalls)
}

func TestContactsAdminMove_NotFound(t *testing.T) {
	stub := &stubContactService{moveErr: errs.ErrContactNotFound}
	router := setupContactAdminRouter(t, stub)

	rec := doRequest(t, router, http.MethodPost,
		"/api/admin/contacts/"+uuid.NewString()+"/move", `{"direction":"up"}`)

	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Equal(t, "Contact not found", decodeError(t, rec.Body.Bytes()).Error.Message)
}

func TestContactsAdminMove_ValidationError(t *testing.T) {
	stub := &stubContactService{moveErr: errs.NewValidationError("direction", "unknown direction")}
	router := setupContactAdminRouter(t, stub)

	rec := doRequest(t, router, http.MethodPost,
		"/api/admin/contacts/"+uuid.NewString()+"/move", `{"direction":"left"}`)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Equal(t, "unknown direction", decodeError(t, rec.Body.Bytes()).Error.Message)
}

func TestContactsAdmin_MiddlewareBlocksAllRoutes(t *testing.T) {
	block := func(c *gin.Context) { c.AbortWithStatus(http.StatusUnauthorized) }
	stub := &stubContactService{items: []*models.Contact{
		contactFixture(uuid.New(), "email", "info@mandat.ua", "", 1),
	}}
	router := setupContactAdminRouter(t, stub, block)
	id := uuid.NewString()

	getRec := doRequest(t, router, http.MethodGet, "/api/admin/contacts", "")
	require.Equal(t, http.StatusUnauthorized, getRec.Code, "the middleware must guard GET")

	postRec := doRequest(t, router, http.MethodPost, "/api/admin/contacts",
		`{"type":"email","value":"info@mandat.ua"}`)
	require.Equal(t, http.StatusUnauthorized, postRec.Code, "the middleware must guard POST")
	assert.Zero(t, stub.createCalls)

	putRec := doRequest(t, router, http.MethodPut, "/api/admin/contacts/"+id,
		`{"type":"email","value":"info@mandat.ua"}`)
	require.Equal(t, http.StatusUnauthorized, putRec.Code, "the middleware must guard PUT")
	assert.Zero(t, stub.updateCalls)

	deleteRec := doRequest(t, router, http.MethodDelete, "/api/admin/contacts/"+id, "")
	require.Equal(t, http.StatusUnauthorized, deleteRec.Code, "the middleware must guard DELETE")
	assert.Zero(t, stub.deleteCalls)

	moveRec := doRequest(t, router, http.MethodPost,
		"/api/admin/contacts/"+id+"/move", `{"direction":"up"}`)
	require.Equal(t, http.StatusUnauthorized, moveRec.Code, "the middleware must guard move")
	assert.Zero(t, stub.moveCalls)
}

func TestContactsPublicList_BypassesAdminMiddleware(t *testing.T) {
	block := func(c *gin.Context) { c.AbortWithStatus(http.StatusUnauthorized) }
	stub := &stubContactService{items: []*models.Contact{
		contactFixture(uuid.New(), "email", "info@mandat.ua", "Редакція", 1),
	}}
	router := setupContactAdminRouter(t, stub, block)

	publicRec := doRequest(t, router, http.MethodGet, "/api/contacts", "")

	require.Equal(t, http.StatusOK, publicRec.Code,
		"the public route must stay open when admin middleware is configured")
	payload := decodeContactList(t, publicRec.Body.Bytes())
	require.Len(t, payload.Items, 1)
	assert.Equal(t, stub.items[0].ID, payload.Items[0].ID)
	assert.Equal(t, "email", payload.Items[0].Type)
	assert.Equal(t, "info@mandat.ua", payload.Items[0].Value)

	adminRec := doRequest(t, router, http.MethodGet, "/api/admin/contacts", "")

	require.Equal(t, http.StatusUnauthorized, adminRec.Code,
		"the same middleware must still guard the admin route")
}
