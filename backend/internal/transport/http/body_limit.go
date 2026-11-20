package httptransport

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/transport/http/routes"
)

// defaultMaxBodyBytes is the maximum accepted request body size.
// maxUploadBytes caps admin image uploads (news covers, ad format cards): 10 MB of image plus
// multipart framing overhead. Everything else keeps the tight limit.
const (
	defaultMaxBodyBytes = 1 << 20
	maxUploadBytes      = 11 << 20
)

// limitBodySize rejects request bodies larger than limit.
func limitBodySize(limit int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		effective := limit
		if isCoverUpload(c) {
			effective = maxUploadBytes
		}

		if c.Request.ContentLength > effective {
			WriteError(c, errs.ErrPayloadTooLarge)

			return
		}

		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, effective)
		c.Next()
	}
}

// isCoverUpload reports multipart admin news and ad format writes, the only
// requests taking files. Plain JSON on the same routes keeps the tight limit.
// Media types are case-insensitive, so the header is lowered first.
func isCoverUpload(c *gin.Context) bool {
	if c.Request.Method != http.MethodPost && c.Request.Method != http.MethodPut {
		return false
	}
	if !strings.HasPrefix(strings.ToLower(c.GetHeader("Content-Type")), "multipart/form-data") {
		return false
	}

	path := c.FullPath()

	return strings.HasPrefix(path, routes.AdminNews) || strings.HasPrefix(path, routes.AdminAdFormats)
}
