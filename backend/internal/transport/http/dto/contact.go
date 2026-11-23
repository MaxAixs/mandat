package dto

import (
	"github.com/google/uuid"
	"github.com/maxaixs/mandat/backend/internal/models"
)

// ContactRequest is the body of the admin contact create and update endpoints.
type ContactRequest struct {
	Type  string `json:"type"`
	Value string `json:"value"`
	Label string `json:"label"`
}

// ContactResponse is a contact as returned by the API; Display and Href are server-built.
type ContactResponse struct {
	ID       uuid.UUID `json:"id"`
	Type     string    `json:"type"`
	Value    string    `json:"value"`
	Label    string    `json:"label"`
	Display  string    `json:"display"`
	Href     string    `json:"href"`
	Position int       `json:"position"`
}

// ContactListResponse is the full ordered list of contacts.
type ContactListResponse struct {
	Items []*ContactResponse `json:"items"`
}

// ContactMoveRequest is the body of POST /api/admin/contacts/:id/move.
type ContactMoveRequest struct {
	Direction string `json:"direction"`
}

// ContactToResponse maps a domain contact onto its API shape.
func ContactToResponse(contact *models.Contact) *ContactResponse {
	return &ContactResponse{
		ID:       contact.ID,
		Type:     string(contact.Type),
		Value:    contact.Value,
		Label:    contact.Label,
		Display:  contact.Display,
		Href:     contact.Href,
		Position: contact.Position,
	}
}
