// Package dto contains the request and response payloads of the HTTP API.
package dto

import "github.com/maxaixs/mandat/backend/internal/service"

// NewsRequest is the body of create and update requests.
// Business rules (published text, rubrics) live in the service; the slug is derived from the title.
type NewsRequest struct {
	Category    string `json:"category" binding:"required"`
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
	Text        string `json:"text"`
	Language    string `json:"language"`
	Status      string `json:"status"`
	Featured    bool   `json:"featured"`
	MainEvent   bool   `json:"main_event"`
	ImageUrl    string `json:"image_url"`
}

// ToInput maps the request body onto the service input.
func (r NewsRequest) ToInput() service.NewsInput {
	return service.NewsInput{
		Category:    r.Category,
		Title:       r.Title,
		Description: r.Description,
		Text:        r.Text,
		Language:    r.Language,
		Status:      r.Status,
		Featured:    r.Featured,
		MainEvent:   r.MainEvent,
		ImageUrl:    r.ImageUrl,
	}
}
