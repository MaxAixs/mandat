package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubRunningLineService returns fixed rates or a fixed error.
type stubRunningLineService struct {
	rates []*models.Rate
	err   error
}

func (s *stubRunningLineService) GetRates(context.Context) ([]*models.Rate, error) {
	return s.rates, s.err
}

// setupRunningLineRouter mounts both running line routes on a bare gin engine.
func setupRunningLineRouter(exchangeSvc, fuelSvc RunningLineService) http.Handler {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	NewRunningLineHandler(exchangeSvc, fuelSvc).Register(engine.Group(""))

	return engine
}

type exchangePayload struct {
	Base  string `json:"base"`
	Date  string `json:"date"`
	Items []struct {
		Code string  `json:"code"`
		Rate float64 `json:"rate"`
	} `json:"items"`
}

type fuelPayload struct {
	Base  string `json:"base"`
	Unit  string `json:"unit"`
	Date  string `json:"date"`
	Items []struct {
		Code string  `json:"code"`
		Rate float64 `json:"rate"`
	} `json:"items"`
}

func TestRunningLineHandler_GetExchangeRates(t *testing.T) {
	exchangeSvc := &stubRunningLineService{rates: []*models.Rate{
		{Code: "EUR", Rate: 50.4823, Date: "10.11.2026"},
		{Code: "USD", Rate: 45.322, Date: "10.11.2026"},
		{Code: "PLN", Rate: 11.4678, Date: "10.11.2026"},
	}}
	// The fuel service deliberately returns different data: each route must
	// read from its own dependency.
	fuelSvc := &stubRunningLineService{rates: []*models.Rate{
		{Code: "LEAK", Rate: 1.234, Date: "01.01.2000"},
	}}
	router := setupRunningLineRouter(exchangeSvc, fuelSvc)

	rec := doRequest(t, router, http.MethodGet, "/api/exchange/rates", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var payload exchangePayload
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))

	assert.Equal(t, "UAH", payload.Base)
	assert.Equal(t, "10.11.2026", payload.Date)
	require.Len(t, payload.Items, 3)

	wantCodes := []string{"EUR", "USD", "PLN"}
	wantRates := []float64{50.48, 45.32, 11.47}

	for i, code := range wantCodes {
		assert.Equal(t, code, payload.Items[i].Code, "rates must be ordered EUR, USD, PLN")
		assert.InDelta(t, wantRates[i], payload.Items[i].Rate, 1e-9, "rates must be rounded to 2 decimals")
	}
}

func TestRunningLineHandler_GetFuelRates(t *testing.T) {
	fuelSvc := &stubRunningLineService{rates: []*models.Rate{
		{Code: "A95", Rate: 88.744, Date: "11.11.2026"},
		{Code: "A92", Rate: 84.684, Date: "11.11.2026"},
		{Code: "A98", Rate: 92.476, Date: "11.11.2026"},
		{Code: "DIESEL", Rate: 104.314, Date: "11.11.2026"},
		{Code: "LPG", Rate: 61.154, Date: "11.11.2026"},
	}}
	// The exchange service deliberately returns different data: each route must
	// read from its own dependency.
	exchangeSvc := &stubRunningLineService{rates: []*models.Rate{
		{Code: "LEAK", Rate: 1.234, Date: "01.01.2000"},
	}}
	router := setupRunningLineRouter(exchangeSvc, fuelSvc)

	rec := doRequest(t, router, http.MethodGet, "/api/fuel/rates", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var payload fuelPayload
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))

	assert.Equal(t, "UAH", payload.Base)
	assert.Equal(t, "liter", payload.Unit)
	assert.Equal(t, "11.11.2026", payload.Date)
	require.Len(t, payload.Items, 5)

	wantCodes := []string{"A95", "A92", "A98", "DIESEL", "LPG"}
	wantRates := []float64{88.74, 84.68, 92.48, 104.31, 61.15}

	for i, code := range wantCodes {
		assert.Equal(t, code, payload.Items[i].Code, "rates must be ordered A95, A92, A98, DIESEL, LPG")
		assert.InDelta(t, wantRates[i], payload.Items[i].Rate, 1e-9, "rates must be rounded to 2 decimals")
	}
}

func TestRunningLineHandler_ExchangeUnavailable(t *testing.T) {
	exchangeSvc := &stubRunningLineService{err: errs.ErrServiceUnavailable}
	router := setupRunningLineRouter(exchangeSvc, &stubRunningLineService{})

	rec := doRequest(t, router, http.MethodGet, "/api/exchange/rates", "")

	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
}

func TestRunningLineHandler_FuelUnavailable(t *testing.T) {
	fuelSvc := &stubRunningLineService{err: errs.ErrServiceUnavailable}
	router := setupRunningLineRouter(&stubRunningLineService{}, fuelSvc)

	rec := doRequest(t, router, http.MethodGet, "/api/fuel/rates", "")

	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
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
