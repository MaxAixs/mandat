package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/maxaixs/mandat/backend/internal/service"
	httptransport "github.com/maxaixs/mandat/backend/internal/transport/http"
	"github.com/maxaixs/mandat/backend/internal/transport/http/handler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubPinger records readiness checks and returns a fixed error.
type stubPinger struct {
	calls int
	err   error
}

func (p *stubPinger) Ping(context.Context) error {
	p.calls++

	return p.err
}

func setupHealthRouter(pinger service.Pinger) http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := service.NewHealthService(pinger, logger)

	httpSrv := httptransport.New(httptransport.HTTPConfig{}, handler.NewHealthHandler(svc))

	return httpSrv.Handler()
}

func decodeHealthStatus(t *testing.T, body []byte) string {
	t.Helper()

	var payload struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(body, &payload))

	return payload.Status
}

func TestHealthLive_Returns200WithoutCheckingDependencies(t *testing.T) {
	pinger := &stubPinger{err: errors.New("database is down")}
	router := setupHealthRouter(pinger)

	rec := doRequest(t, router, http.MethodGet, "/health/live", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "ok", decodeHealthStatus(t, rec.Body.Bytes()))
	assert.Zero(t, pinger.calls, "liveness must not ping its dependencies")
}

func TestHealthReady_Returns200WhenDependencyIsUp(t *testing.T) {
	pinger := &stubPinger{}
	router := setupHealthRouter(pinger)

	rec := doRequest(t, router, http.MethodGet, "/health/ready", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "ok", decodeHealthStatus(t, rec.Body.Bytes()))
	assert.Equal(t, 1, pinger.calls, "readiness must ping the dependency exactly once")
}

func TestHealthReady_Returns503WhenDependencyIsDown(t *testing.T) {
	pinger := &stubPinger{err: errors.New("dial tcp 10.0.0.5:5432: connection refused")}
	router := setupHealthRouter(pinger)

	rec := doRequest(t, router, http.MethodGet, "/health/ready", "")

	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
	body := decodeError(t, rec.Body.Bytes())
	assert.Equal(t, http.StatusServiceUnavailable, body.Error.Status)
	assert.Equal(t, "Service unavailable", body.Error.Message)
	assert.NotContains(t, rec.Body.String(), "10.0.0.5",
		"the response must not leak dependency details")
	assert.Equal(t, 1, pinger.calls)
}
