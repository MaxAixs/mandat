// Package routes holds the URL prefixes shared by handlers and middleware.
// It has no dependencies, so any transport package can import it without cycles.
package routes

const (
	// API is the root of every JSON endpoint.
	API = "/api"
	// AdminBase is the root of the staff-only API.
	AdminBase = API + "/admin"
	// AdminNews is the root of the news administration endpoints.
	AdminNews = AdminBase + "/news"
	// Auth is the root of the staff authentication endpoints.
	Auth = API + "/auth"
	// PublicNews is the root of the public news endpoints.
	PublicNews = API + "/news"
	// Health is the root of the liveness and readiness probes.
	Health = "/health"
)
