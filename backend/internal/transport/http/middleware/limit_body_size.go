// Package middleware holds the gin middlewares of the HTTP layer.
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// DefaultMaxBodyBytes is the maximum accepted request body size.
// maxUploadBytes caps admin cover uploads: 10 MB of image plus
// multipart framing overhead. Everything else keeps the tight limit.
const (
	DefaultMaxBodyBytes = 1 << 20
	maxUploadBytes      = 11 << 20
)

// uploadPathPrefix marks the admin routes that accept cover files.
// It mirrors the admin news routes in the handler package: importing them
// would cycle through httptransport, so the prefix is repeated here.
const uploadPathPrefix = "/api/admin/news"

// LimitBodySize rejects request bodies larger than limit.
func LimitBodySize(limit int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		effective := limit
		if isCoverUpload(c) {
			effective = maxUploadBytes
		}

		if c.Request.ContentLength > effective {
			// Same wire format as WriteError for ErrPayloadTooLarge.
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"error": gin.H{
				"status":  http.StatusRequestEntityTooLarge,
				"message": "Request payload too large",
			}})

			return
		}

		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, effective)
		c.Next()
	}
}

// isCoverUpload reports multipart admin news writes, the only requests
// taking files. Plain JSON on the same routes keeps the tight limit.
// Media types are case-insensitive, so the header is lowered first.
func isCoverUpload(c *gin.Context) bool {
	if c.Request.Method != http.MethodPost && c.Request.Method != http.MethodPut {
		return false
	}
	if !strings.HasPrefix(strings.ToLower(c.GetHeader("Content-Type")), "multipart/form-data") {
		return false
	}

	return strings.HasPrefix(c.FullPath(), uploadPathPrefix)
}
