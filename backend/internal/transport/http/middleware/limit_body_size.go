// Package middleware holds the gin middlewares of the HTTP layer.
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/transport/http/handler"
)

// LimitBodySize rejects request bodies larger than limit.
func LimitBodySize(limit int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > limit {
			handler.WriteError(c, errs.ErrPayloadTooLarge)
			return
		}

		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		c.Next()
	}
}
