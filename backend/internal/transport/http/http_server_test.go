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
	srv, err := New(testConfig(), nil, stubRegistrar{path: "/ping"})
	require.NoError(t, err)

	assert.Equal(t, ":9099", srv.server.Addr)
	assert.Equal(t, 1*time.Second, srv.server.ReadHeaderTimeout)
	assert.Equal(t, 2*time.Second, srv.server.ReadTimeout)
	assert.Equal(t, 3*time.Second, srv.server.WriteTimeout)
	assert.Equal(t, 4*time.Second, srv.server.IdleTimeout)
	assert.Equal(t, 5*time.Second, srv.shutdownTimeout)
}

func TestNew_ServesRegisteredRoutes(t *testing.T) {
	srv, err := New(testConfig(), nil, stubRegistrar{path: "/ping"})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ping", nil))

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestShutdown_SafeOnNotStartedServer(t *testing.T) {
	srv, err := New(testConfig(), nil, stubRegistrar{path: "/ping"})
	require.NoError(t, err)

	require.NoError(t, srv.Shutdown())
}

// ipEchoRegistrar answers with gin's client IP so tests observe which
// address New wired behind trusted proxies and IP headers.
type ipEchoRegistrar struct{}

func (ipEchoRegistrar) Register(r *gin.RouterGroup) {
	r.GET("/whoami", func(c *gin.Context) {
		c.String(http.StatusOK, c.ClientIP())
	})
}

func doServerRequest(
	t *testing.T,
	srv *Server,
	method string,
	path string,
	remoteAddr string,
	headers map[string]string,
) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, nil)
	req.RemoteAddr = remoteAddr
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	return rec
}

func TestNew_TrustedProxyHonoursClientIPHeader(t *testing.T) {
	cfg := testConfig()
	cfg.TrustedProxies = []string{"10.0.0.0/8"}
	cfg.ClientIPHeaders = []string{"CF-Connecting-IP"}

	srv, err := New(cfg, nil, ipEchoRegistrar{})
	require.NoError(t, err)

	rec := doServerRequest(t, srv, http.MethodGet, "/whoami", "10.1.2.3:1234", map[string]string{
		"CF-Connecting-IP": "203.0.113.9",
	})

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "203.0.113.9", rec.Body.String(),
		"a request from a trusted proxy must resolve the client from CF-Connecting-IP")
}

func TestNew_UntrustedProxyIgnoresSpoofedHeaders(t *testing.T) {
	cfg := testConfig()
	cfg.ClientIPHeaders = []string{"CF-Connecting-IP"}

	srv, err := New(cfg, nil, ipEchoRegistrar{})
	require.NoError(t, err)

	rec := doServerRequest(t, srv, http.MethodGet, "/whoami", "10.1.2.3:1234", map[string]string{
		"CF-Connecting-IP": "203.0.113.9",
		"X-Forwarded-For":  "198.51.100.7",
	})

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "10.1.2.3", rec.Body.String(),
		"with empty trusted_proxies no header may override the peer address")
}

func TestNew_InvalidTrustedProxyReturnsError(t *testing.T) {
	cfg := testConfig()
	cfg.TrustedProxies = []string{"not-a-cidr"}

	srv, err := New(cfg, nil, stubRegistrar{path: "/ping"})

	require.Error(t, err, "an invalid trusted proxy entry must fail server construction")
	assert.Nil(t, srv)
}

func TestNew_GlobalMiddlewareRunsOnRoutes(t *testing.T) {
	seen := false
	probe := func(c *gin.Context) {
		seen = true
		c.Writer.Header().Set("X-Test-Middleware", "applied")
		c.Next()
	}

	srv, err := New(testConfig(), []gin.HandlerFunc{probe}, stubRegistrar{path: "/ping"})
	require.NoError(t, err)

	rec := doServerRequest(t, srv, http.MethodGet, "/ping", "10.1.2.3:1234", nil)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.True(t, seen, "a global middleware passed to New must run on registered routes")
	assert.Equal(t, "applied", rec.Header().Get("X-Test-Middleware"))
}

func TestNew_RejectionStillCarriesCORSHeaders(t *testing.T) {
	cfg := testConfig()
	cfg.AllowedOrigins = []string{"https://news.example.com"}
	reject := func(c *gin.Context) {
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": gin.H{
			"status":  http.StatusTooManyRequests,
			"message": "Too many requests",
		}})
	}

	srv, err := New(cfg, []gin.HandlerFunc{reject}, stubRegistrar{path: "/ping"})
	require.NoError(t, err)

	rec := doServerRequest(t, srv, http.MethodGet, "/ping", "10.1.2.3:1234", map[string]string{
		"Origin": "https://news.example.com",
	})

	require.Equal(t, http.StatusTooManyRequests, rec.Code, rec.Body.String())
	assert.Equal(t, "https://news.example.com", rec.Header().Get("Access-Control-Allow-Origin"),
		"a throttled browser client still needs its CORS headers")
}

func TestNew_PlainIPTrustedProxyHonoursClientIPHeader(t *testing.T) {
	cfg := testConfig()
	cfg.TrustedProxies = []string{"10.1.2.3"}
	cfg.ClientIPHeaders = []string{"CF-Connecting-IP"}

	srv, err := New(cfg, nil, ipEchoRegistrar{})
	require.NoError(t, err)

	rec := doServerRequest(t, srv, http.MethodGet, "/whoami", "10.1.2.3:1234", map[string]string{
		"CF-Connecting-IP": "203.0.113.9",
	})

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "203.0.113.9", rec.Body.String(),
		"a plain-IP trusted proxy must resolve the client from CF-Connecting-IP")
}

func TestNew_EmptyClientIPHeadersHonoursNoHeaders(t *testing.T) {
	cfg := testConfig()
	cfg.TrustedProxies = []string{"10.0.0.0/8"}
	cfg.ClientIPHeaders = nil

	srv, err := New(cfg, nil, ipEchoRegistrar{})
	require.NoError(t, err)

	rec := doServerRequest(t, srv, http.MethodGet, "/whoami", "10.1.2.3:1234", map[string]string{
		"X-Forwarded-For": "203.0.113.9",
		"X-Real-IP":       "203.0.113.9",
	})

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "10.1.2.3", rec.Body.String(),
		"an empty client_ip_headers list must honour no header, even from a trusted proxy")
}
