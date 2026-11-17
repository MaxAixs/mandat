package httptransport

import (
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// corsMiddleware allows cross-origin requests only from the given origins.
// Staff authenticate with a cookie, so no Authorization header is allowed.
func corsMiddleware(origins []string) gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOrigins: origins,
		AllowMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept"},
		AllowCredentials: true,
	})
}
