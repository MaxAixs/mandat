package httptransport

import (
	"encoding/json"
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

// guardProbeRegistrar exposes staff and public routes so the server-level
// guards can be observed through New without involving real handlers.
type guardProbeRegistrar struct {
	called *bool
}

func (g guardProbeRegistrar) Register(r *gin.RouterGroup) {
	h := func(c *gin.Context) {
		*g.called = true
		c.String(http.StatusOK, "ok")
	}

	r.POST("/api/admin/news", h)
	r.PUT("/api/admin/news/:slug", h)
	r.PATCH("/api/admin/news/:slug", h)
	r.DELETE("/api/admin/news/:slug", h)
	r.GET("/api/admin/news", h)
	r.POST("/api/auth/login", h)
	r.POST("/api/auth/logout-all", h)
	r.GET("/api/auth/me", h)
	r.GET("/api/news/x", h)
	r.POST("/api/news/x/view", h)
}

func newGuardServer(
	t *testing.T,
	cfg HTTPConfig,
	called *bool,
) *Server {
	t.Helper()

	srv, err := New(cfg, nil, guardProbeRegistrar{called: called})
	require.NoError(t, err)

	return srv
}

func doGuardRequest(
	t *testing.T,
	srv *Server,
	method string,
	path string,
	host string,
	origin string,
) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, nil)
	if host != "" {
		req.Host = host
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	return rec
}

func TestSecurityHeaders_AppliedToEveryResponse(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"staff post", http.MethodPost, "/api/admin/news"},
		{"auth login", http.MethodPost, "/api/auth/login"},
		{"public get", http.MethodGet, "/api/news/x"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			srv := newGuardServer(t, testConfig(), &called)

			rec := doGuardRequest(t, srv, tt.method, tt.path, "", "")

			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
			assert.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
			assert.Equal(t, "strict-origin-when-cross-origin", rec.Header().Get("Referrer-Policy"))
			assert.Equal(t, "default-src 'none'; frame-ancestors 'none'",
				rec.Header().Get("Content-Security-Policy"))
		})
	}
}

func TestSecurityHeaders_HSTSOnlyWhenSecureCookies(t *testing.T) {
	tests := []struct {
		name          string
		secureCookies bool
		wantHSTS      string
	}{
		{"secure cookies set HSTS", true, "max-age=31536000; includeSubDomains"},
		{"plain cookies omit HSTS", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			cfg := testConfig()
			cfg.SecureCookies = tt.secureCookies
			srv := newGuardServer(t, cfg, &called)

			rec := doGuardRequest(t, srv, http.MethodGet, "/api/news/x", "", "")

			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			assert.Equal(t, tt.wantHSTS, rec.Header().Get("Strict-Transport-Security"))
		})
	}
}

func TestCacheControl_NoStoreOnStaffPaths(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		want   string
	}{
		{"admin path", http.MethodGet, "/api/admin/news", "no-store"},
		{"auth path", http.MethodPost, "/api/auth/login", "no-store"},
		{"public path untouched", http.MethodGet, "/api/news/x", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			srv := newGuardServer(t, testConfig(), &called)

			rec := doGuardRequest(t, srv, tt.method, tt.path, "", "")

			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			assert.Equal(t, tt.want, rec.Header().Get("Cache-Control"))
		})
	}
}

func TestOriginGuard_BlocksForeignStateChangingStaffRequests(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"admin post", http.MethodPost, "/api/admin/news"},
		{"admin put", http.MethodPut, "/api/admin/news/some-slug"},
		{"admin patch", http.MethodPatch, "/api/admin/news/some-slug"},
		{"admin delete", http.MethodDelete, "/api/admin/news/some-slug"},
		{"auth login", http.MethodPost, "/api/auth/login"},
		{"auth logout-all", http.MethodPost, "/api/auth/logout-all"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			cfg := testConfig()
			cfg.AllowedOrigins = nil
			srv := newGuardServer(t, cfg, &called)

			rec := doGuardRequest(t, srv, tt.method, tt.path, "api.mandat.test", "https://evil.example")

			require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
			assert.False(t, called, "a rejected cross-site write must not reach the handler")

			var payload struct {
				Error struct {
					Status  int    `json:"status"`
					Message string `json:"message"`
				} `json:"error"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
			assert.Equal(t, http.StatusForbidden, payload.Error.Status)
			assert.Equal(t, "Untrusted request origin", payload.Error.Message)
		})
	}
}

func TestOriginGuard_AllowsLegitimateRequests(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		path           string
		host           string
		origin         string
		allowedOrigins []string
	}{
		{"no origin passes", http.MethodPost, "/api/admin/news", "api.mandat.test", "",
			[]string{"http://localhost:5173"}},
		{"listed origin passes", http.MethodPost, "/api/admin/news", "api.mandat.test",
			"http://localhost:5173", []string{"http://localhost:5173"}},
		{"same host passes without allowlist", http.MethodPost, "/api/admin/news",
			"api.mandat.test", "https://api.mandat.test", nil},
		{"get with foreign origin passes", http.MethodGet, "/api/admin/news",
			"api.mandat.test", "https://evil.example", nil},
		{"public post with foreign origin passes", http.MethodPost, "/api/news/x/view",
			"api.mandat.test", "https://evil.example", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			cfg := testConfig()
			cfg.AllowedOrigins = tt.allowedOrigins
			srv := newGuardServer(t, cfg, &called)

			rec := doGuardRequest(t, srv, tt.method, tt.path, tt.host, tt.origin)

			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			assert.True(t, called, "a legitimate request must reach the handler")
		})
	}
}

func TestCORS_PreflightExcludesAuthorizationHeader(t *testing.T) {
	cfg := testConfig()
	cfg.AllowedOrigins = []string{"http://localhost:5173"}
	called := false
	srv := newGuardServer(t, cfg, &called)

	req := httptest.NewRequest(http.MethodOptions, "/api/admin/news", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "Authorization")

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	allowHeaders := rec.Header().Get("Access-Control-Allow-Headers")
	assert.NotContains(t, allowHeaders, "Authorization",
		"staff cookies must not be replaced by an Authorization header")
	assert.Contains(t, allowHeaders, "Content-Type")
}
