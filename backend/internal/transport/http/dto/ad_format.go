package dto

import (
	"github.com/google/uuid"
	"github.com/maxaixs/mandat/backend/internal/models"
)

// AdFormatResponse is an ad format card as returned by the API.
type AdFormatResponse struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	ImageURL    string    `json:"image_url"`
	Position    int       `json:"position"`
}

// AdFormatListResponse is the full ordered list of cards.
type AdFormatListResponse struct {
	Items []*AdFormatResponse `json:"items"`
}

// AdFormatMoveRequest is the body of POST /api/admin/ad-formats/:id/move.
type AdFormatMoveRequest struct {
	Direction string `json:"direction"`
}

// AdFormatToResponse maps a domain card onto its API shape.
func AdFormatToResponse(format *models.AdFormat) *AdFormatResponse {
	return &AdFormatResponse{
		ID:          format.ID,
		Title:       format.Title,
		Description: format.Description,
		ImageURL:    format.ImageURL,
		Position:    format.Position,
	}
}
