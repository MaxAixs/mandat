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

// stubExchangeService returns fixed rates or a fixed error.
type stubExchangeService struct {
	rates []*models.ExchangeRate
	err   error
}

func (s *stubExchangeService) GetRates(context.Context) ([]*models.ExchangeRate, error) {
	return s.rates, s.err
}

// setupExchangeRouter mounts the exchange routes on a bare gin engine.
func setupExchangeRouter(svc ExchangeService) http.Handler {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	NewExchangeHandler(svc).Register(engine.Group(""))

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

func TestExchangeHandler_GetRates(t *testing.T) {
	svc := &stubExchangeService{rates: []*models.ExchangeRate{
		{Code: "EUR", Rate: 50.4823, Date: "10.11.2026"},
		{Code: "USD", Rate: 45.322, Date: "10.11.2026"},
		{Code: "PLN", Rate: 11.4678, Date: "10.11.2026"},
	}}
	router := setupExchangeRouter(svc)

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

func TestExchangeHandler_ServiceUnavailable(t *testing.T) {
	svc := &stubExchangeService{err: errs.ErrServiceUnavailable}
	router := setupExchangeRouter(svc)

	rec := doRequest(t, router, http.MethodGet, "/api/exchange/rates", "")

	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
}
