package httptransport

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubRegistrar registers one test route to prove routing works after New.
type stubRegistrar struct{ path string }

func (sr stubRegistrar) Register(r *gin.RouterGroup) {
	r.GET(sr.path, func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
}

func testConfig() HTTPConfig {
	return HTTPConfig{
		Port:              9099,
		ReadHeaderTimeout: 1 * time.Second,
		ReadTimeout:       2 * time.Second,
		WriteTimeout:      3 * time.Second,
		IdleTimeout:       4 * time.Second,
		ShutdownTimeout:   5 * time.Second,
	}
}

func TestNew_AppliesConfiguredTimeouts(t *testing.T) {
	srv := New(testConfig(), stubRegistrar{path: "/ping"})

	assert.Equal(t, ":9099", srv.server.Addr)
	assert.Equal(t, 1*time.Second, srv.server.ReadHeaderTimeout)
	assert.Equal(t, 2*time.Second, srv.server.ReadTimeout)
	assert.Equal(t, 3*time.Second, srv.server.WriteTimeout)
	assert.Equal(t, 4*time.Second, srv.server.IdleTimeout)
	assert.Equal(t, 5*time.Second, srv.shutdownTimeout)
}

func TestNew_ServesRegisteredRoutes(t *testing.T) {
	srv := New(testConfig(), stubRegistrar{path: "/ping"})

	rec := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ping", nil))

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestShutdown_SafeOnNotStartedServer(t *testing.T) {
	srv := New(testConfig(), stubRegistrar{path: "/ping"})

	require.NoError(t, srv.Shutdown())
}
