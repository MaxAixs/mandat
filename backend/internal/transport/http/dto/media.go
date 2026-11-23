package dto

import (
	"github.com/maxaixs/mandat/backend/internal/models"
)

// VideoResponse is the video attached to an article.
type VideoResponse struct {
	Kind string `json:"kind"`
	URL  string `json:"url"`
}

// VideoToResponse maps an optional article video; nil stays nil (JSON null).
func VideoToResponse(video *models.Video) *VideoResponse {
	if video == nil {
		return nil
	}

	return &VideoResponse{Kind: string(video.Kind), URL: video.URL}
}

// ImagesToResponse maps the article images onto a JSON array; none renders as [].
func ImagesToResponse(imageURLs []string) []string {
	if imageURLs == nil {
		return []string{}
	}

	return imageURLs
}
