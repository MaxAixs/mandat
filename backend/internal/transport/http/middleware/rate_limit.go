package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/maxaixs/mandat/backend/internal/errs"
	httptransport "github.com/maxaixs/mandat/backend/internal/transport/http"
	"github.com/maxaixs/mandat/backend/internal/transport/http/routes"
)

// ipv6SubscriberBits is the prefix one IPv6 subscriber usually owns,
// so rotating addresses inside it does not reset the limit.
const ipv6SubscriberBits = 64

// RateLimiter decides whether a client may make another request.
type RateLimiter interface {
	Allow(ctx context.Context, key string) (time.Duration, error)
}

// RateLimit rejects clients that exceeded the global request limit.
// Limiter failures other than the limit itself let the request through.
func RateLimit(limiter RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isRateLimitExempt(c) {
			c.Next()

			return
		}

		wait, err := limiter.Allow(c.Request.Context(), clientKey(c.ClientIP()))
		if errors.Is(err, errs.ErrTooManyRequests) {
			httptransport.WriteError(c, errs.NewRetryAfterError(wait))

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

	return path == routes.Health || strings.HasPrefix(path, routes.Health+"/")
}

// clientKey identifies a client: IPv4 as is, IPv6 by its /64 prefix,
// anything unparsable verbatim.
func clientKey(clientIP string) string {
	addr, err := netip.ParseAddr(clientIP)
	if err != nil {
		return clientIP
	}

	addr = addr.Unmap().WithZone("")
	if addr.Is4() {
		return addr.String()
	}

	return netip.PrefixFrom(addr, ipv6SubscriberBits).Masked().String()
}
