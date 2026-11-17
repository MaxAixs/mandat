package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rateLimitFakeLimiter stands in for the service so tests need no Redis.
type rateLimitFakeLimiter struct {
	calls   int
	lastKey string
	wait    time.Duration
	err     error
}

// Allow records the client key and answers with the canned wait or error.
func (f *rateLimitFakeLimiter) Allow(_ context.Context, key string) (time.Duration, error) {
	f.calls++
	f.lastKey = key

	return f.wait, f.err
}

// overLimitErr wraps the sentinel the way the service does.
func overLimitErr() error {
	return fmt.Errorf("rateLimitService_Allow: limit exceeded: %w", errs.ErrTooManyRequests)
}

func TestRateLimit_AllowedPassesToHandler(t *testing.T) {
	limiter := &rateLimitFakeLimiter{}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RateLimit(limiter))
	router.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.RemoteAddr = "203.0.113.9:1234"
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "ok", rec.Body.String())
	assert.Equal(t, 1, limiter.calls, "an allowed request must consult the limiter exactly once")
}

func TestRateLimit_OverLimitReturns429(t *testing.T) {
	tests := []struct {
		name           string
		wait           time.Duration
		wantRetryAfter string
	}{
		{"fraction rounds up", 1200 * time.Millisecond, "2"},
		{"zero waits one second", 0, "1"},
		{"whole seconds pass through", 60 * time.Second, "60"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter := &rateLimitFakeLimiter{wait: tt.wait, err: overLimitErr()}
			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.Use(RateLimit(limiter))
			router.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

			req := httptest.NewRequest(http.MethodGet, "/ping", nil)
			req.RemoteAddr = "203.0.113.9:1234"
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusTooManyRequests, rec.Code, rec.Body.String())
			assert.Equal(t, tt.wantRetryAfter, rec.Header().Get("Retry-After"),
				"Retry-After carries the ceiling of the wait, at least one second")

			var payload struct {
				Error struct {
					Status  int    `json:"status"`
					Message string `json:"message"`
				} `json:"error"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
			assert.Equal(t, http.StatusTooManyRequests, payload.Error.Status)
			assert.Equal(t, "Too many requests", payload.Error.Message)
		})
	}
}

func TestRateLimit_OtherErrorFailsOpen(t *testing.T) {
	limiter := &rateLimitFakeLimiter{err: errors.New("redis is down")}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RateLimit(limiter))
	router.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String(),
		"a limiter failure must let the request through")
	assert.Equal(t, "ok", rec.Body.String())
	assert.Equal(t, 1, limiter.calls)
}

func TestRateLimit_SkipsPreflightAndHealth(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"options preflight", http.MethodOptions, "/ping"},
		{"health live probe", http.MethodGet, "/health/live"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter := &rateLimitFakeLimiter{wait: time.Minute, err: overLimitErr()}
			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.Use(RateLimit(limiter))
			router.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
			router.OPTIONS("/ping", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
			router.GET("/health/live", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

			// Control: the same limiter throttles the ordinary route,
			// so the skip below actually proves something.
			control := httptest.NewRecorder()
			router.ServeHTTP(control, httptest.NewRequest(http.MethodGet, "/ping", nil))
			require.Equal(t, http.StatusTooManyRequests, control.Code, control.Body.String())
			require.Equal(t, 1, limiter.calls)

			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String(),
				"probes and preflights are never throttled")
			assert.Equal(t, 1, limiter.calls,
				"the limiter must not run for %s %s", tt.method, tt.path)
		})
	}
}

func TestRateLimit_PassesClientIP(t *testing.T) {
	limiter := &rateLimitFakeLimiter{}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RateLimit(limiter))
	router.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.RemoteAddr = "203.0.113.9:1234"
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, 1, limiter.calls)
	assert.Equal(t, "203.0.113.9", limiter.lastKey,
		"the limiter must key on gin's client IP")
}

func TestRateLimit_SkipsBareHealthAndReady(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"bare health", "/health"},
		{"readiness probe", "/health/ready"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter := &rateLimitFakeLimiter{wait: time.Minute, err: overLimitErr()}
			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.Use(RateLimit(limiter))
			router.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
			router.GET(tt.path, func(c *gin.Context) { c.String(http.StatusOK, "ok") })

			// Control: the same limiter throttles the ordinary route,
			// so the skip below actually proves something.
			control := httptest.NewRecorder()
			router.ServeHTTP(control, httptest.NewRequest(http.MethodGet, "/ping", nil))
			require.Equal(t, http.StatusTooManyRequests, control.Code, control.Body.String())
			require.Equal(t, 1, limiter.calls)

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String(),
				"probes are never throttled")
			assert.Equal(t, 1, limiter.calls,
				"the limiter must not run for GET %s", tt.path)
		})
	}
}

func TestRateLimit_ThrottlesHealthLookalikes(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"no slash suffix", "/healthz"},
		{"longer prefix word", "/healthcheck"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter := &rateLimitFakeLimiter{wait: time.Second, err: overLimitErr()}
			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.Use(RateLimit(limiter))
			router.GET(tt.path, func(c *gin.Context) { c.String(http.StatusOK, "ok") })

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusTooManyRequests, rec.Code, rec.Body.String(),
				"only /health and /health/* are exempt")
			assert.Equal(t, 1, limiter.calls,
				"the limiter must run for GET %s", tt.path)
		})
	}
}

func TestRateLimit_NegativeWaitClampsRetryAfter(t *testing.T) {
	limiter := &rateLimitFakeLimiter{wait: -500 * time.Millisecond, err: overLimitErr()}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RateLimit(limiter))
	router.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusTooManyRequests, rec.Code, rec.Body.String())
	assert.Equal(t, "1", rec.Header().Get("Retry-After"),
		"a negative wait must clamp to the one-second minimum")
}

func TestRateLimit_NormalizesClientKey(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		wantKey    string
	}{
		{"ipv4 verbatim", "203.0.113.7:1234", "203.0.113.7"},
		{"ipv6 first in /64", "[2001:db8:1:2:aaaa::1]:1234", "2001:db8:1:2::/64"},
		{"ipv6 second in /64", "[2001:db8:1:2:bbbb::2]:1234", "2001:db8:1:2::/64"},
		{"ipv4-mapped ipv6", "[::ffff:203.0.113.7]:1234", "203.0.113.7"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter := &rateLimitFakeLimiter{}
			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.Use(RateLimit(limiter))
			router.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

			req := httptest.NewRequest(http.MethodGet, "/ping", nil)
			req.RemoteAddr = tt.remoteAddr
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			require.Equal(t, 1, limiter.calls)
			assert.Equal(t, tt.wantKey, limiter.lastKey)
		})
	}
}

func TestRateLimit_SameSlash64SharesKey(t *testing.T) {
	limiter := &rateLimitFakeLimiter{}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RateLimit(limiter))
	router.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	serve := func(remoteAddr string) {
		t.Helper()

		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.RemoteAddr = remoteAddr
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	}

	serve("[2001:db8:1:2:aaaa::1]:1234")
	require.Equal(t, 1, limiter.calls)
	firstKey := limiter.lastKey

	serve("[2001:db8:1:2:bbbb::2]:1234")
	require.Equal(t, 2, limiter.calls)

	assert.Equal(t, firstKey, limiter.lastKey,
		"two subscribers behind one /64 must share a single counter")
	assert.Equal(t, "2001:db8:1:2::/64", limiter.lastKey)
}

func TestRateLimit_ZonedLinkLocalStripsZone(t *testing.T) {
	limiter := &rateLimitFakeLimiter{}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RateLimit(limiter))
	router.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.RemoteAddr = "[fe80::1%eth0]:1234"
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, 1, limiter.calls)
	if limiter.lastKey == "" {
		t.Skip("gin ClientIP drops the zone, so the middleware never sees a zoned address")
	}
	assert.Equal(t, "fe80::/64", limiter.lastKey)
}
