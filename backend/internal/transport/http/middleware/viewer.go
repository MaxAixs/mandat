package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/maxaixs/mandat/backend/internal/transport/http/dto"
)

const viewerCookieTTL = 365 * 24 * time.Hour

// EnsureViewer assigns a persistent anonymous visitor UUID when needed.
func EnsureViewer(secureCookie bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, cookieErr := c.Cookie(dto.ViewerCookie)
		viewerID, err := uuid.Parse(cookie)
		if cookieErr != nil {
			viewerID = uuid.Nil
		}
		if err != nil || viewerID == uuid.Nil {
			viewerID = uuid.New()
			c.SetSameSite(http.SameSiteLaxMode)
			c.SetCookie(
				dto.ViewerCookie,
				viewerID.String(),
				int(viewerCookieTTL.Seconds()),
				"/api/news",
				"",
				secureCookie,
				true,
			)
		}

		c.Set(dto.ViewerIDKey, viewerID)
		c.Next()
	}
}

// ViewerID returns the anonymous visitor UUID set by EnsureViewer.
func ViewerID(c *gin.Context) uuid.UUID {
	value, ok := c.Get(dto.ViewerIDKey)
	if !ok {
		return uuid.Nil
	}

	viewerID, _ := value.(uuid.UUID)
	return viewerID
}
