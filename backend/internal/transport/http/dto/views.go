package dto

const (
	// ViewerCookie identifies an anonymous public visitor.
	ViewerCookie = "mandat_viewer"
	// ViewerIDKey is the Gin context key set by viewer middleware.
	ViewerIDKey = "anonymousViewerID"
)

// ViewResponse is the current public article view total.
type ViewResponse struct {
	Views int64 `json:"views"`
}
