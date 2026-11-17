package httptransport

import (
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/transport/http/routes"
)

// Baseline security header values for a JSON-only API.
const (
	contentSecurityPolicy = "default-src 'none'; frame-ancestors 'none'"
	referrerPolicy        = "strict-origin-when-cross-origin"
	hstsPolicy            = "max-age=31536000; includeSubDomains"
)

// staffPrefixes are the private API roots: their responses are never cached
// and their state-changing requests must come from a trusted origin.
var staffPrefixes = []string{routes.AdminBase, routes.Auth}

// securityHeaders sets the baseline security headers on every response.
// HSTS is sent only when the deployment serves HTTPS (secure cookies on).
func securityHeaders(hsts bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.Writer.Header()
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("X-Frame-Options", "DENY")
		header.Set("Referrer-Policy", referrerPolicy)
		header.Set("Content-Security-Policy", contentSecurityPolicy)
		if hsts {
			header.Set("Strict-Transport-Security", hstsPolicy)
		}
		if isStaffPath(c.Request.URL.Path) {
			header.Set("Cache-Control", "no-store")
		}

		c.Next()
	}
}

// originGuard rejects state-changing staff requests sent by a browser from
// an untrusted origin. SameSite=Lax alone does not stop same-site subdomains.
// Requests without Origin come from non-browser clients and pass.
func originGuard(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		safeMethod := c.Request.Method == http.MethodGet ||
			c.Request.Method == http.MethodHead ||
			c.Request.Method == http.MethodOptions
		if safeMethod || !isStaffPath(c.Request.URL.Path) {
			c.Next()

			return
		}

		origin := c.GetHeader("Origin")
		if origin == "" || slices.Contains(allowedOrigins, origin) || isSameHost(origin, c.Request.Host) {
			c.Next()

			return
		}

		WriteError(c, errs.ErrUntrustedOrigin)
	}
}

// isStaffPath reports paths under the admin or auth API roots.
func isStaffPath(path string) bool {
	for _, prefix := range staffPrefixes {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}

	return false
}

// isSameHost reports an Origin pointing at the host that serves the request.
func isSameHost(origin, host string) bool {
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}

	return strings.EqualFold(parsed.Host, host)
}
