// Package httptransport wires the HTTP routes of the API and runs the server.
package httptransport

import "github.com/gin-gonic/gin"

// Registrar binds the routes of one implementation to a gin router.
// Handlers satisfy it implicitly by defining a Register method.
type Registrar interface {
	Register(r *gin.RouterGroup)
}
