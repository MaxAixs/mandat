package middleware

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/maxaixs/mandat/backend/internal/errs"
)

// healthPathPrefix marks the probe routes, which are never throttled.
// It mirrors the health handler group: importing it would cycle through httptransport.
const healthPathPrefix = "/health"

// minRetryAfterSeconds keeps Retry-After meaningful when the window is about to reset.
const minRetryAfterSeconds = 1

// RateLimiter decides whether a client may make another request.
type RateLimiter interface {
	Allow(ctx context.Context, clientIP string) (time.Duration, error)
}

// RateLimit rejects clients that exceeded the global request limit.
// Limiter failures other than the limit itself let the request through.
func RateLimit(limiter RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isRateLimitExempt(c) {
			c.Next()

			return
		}

		wait, err := limiter.Allow(c.Request.Context(), c.ClientIP())
		if errors.Is(err, errs.ErrTooManyRequests) {
			c.Header("Retry-After", strconv.Itoa(retryAfterSeconds(wait)))
			// Same wire format as WriteError, written here because WriteError cannot set
			// Retry-After and its errResps message for this sentinel is login-specific.
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": gin.H{
				"status":  http.StatusTooManyRequests,
				"message": "Too many requests",
			}})

			return
		}

		c.Next()
	}
}

// isRateLimitExempt reports CORS preflights and health probes.
func isRateLimitExempt(c *gin.Context) bool {
	if c.Request.Method == http.MethodOptions {
		return true
	}

	path := c.Request.URL.Path

	return path == healthPathPrefix || strings.HasPrefix(path, healthPathPrefix+"/")
}

// retryAfterSeconds rounds wait up to whole seconds, at least minRetryAfterSeconds.
func retryAfterSeconds(wait time.Duration) int {
	seconds := int(math.Ceil(wait.Seconds()))
	if seconds < minRetryAfterSeconds {
		return minRetryAfterSeconds
	}

	return seconds
}
