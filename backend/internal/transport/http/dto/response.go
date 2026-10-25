package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/maxaixs/mandat/backend/internal/models"
)

// NewsResponse is a news article as returned by the API.
type NewsResponse struct {
	ID          uuid.UUID  `json:"id"`
	Slug        string     `json:"slug"`
	Category    string     `json:"category"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Text        string     `json:"text"`
	Language    string     `json:"language"`
	Status      string     `json:"status"`
	Views       int64      `json:"views"`
	Featured    bool       `json:"featured"`
	ImageUrl    string     `json:"image_url"`
	PublishedAt *time.Time `json:"published_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// ListResponse is a page of news articles.
type ListResponse struct {
	Items []*NewsResponse `json:"items"`
	Page  int             `json:"page"`
	Limit int             `json:"limit"`
}

// PublicNewsResponse is a published article card returned by the public API.
type PublicNewsResponse struct {
	Slug        string     `json:"slug"`
	Category    string     `json:"category"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	ImageUrl    string     `json:"image_url"`
	Featured    bool       `json:"featured"`
	Views       int64      `json:"views"`
	PublishedAt *time.Time `json:"published_at"`
}

// PublicNewsListResponse is a page of published article cards.
type PublicNewsListResponse struct {
	Items []*PublicNewsResponse `json:"items"`
	Page  int                   `json:"page"`
	Limit int                   `json:"limit"`
}

// HealthResponse is the successful response of a health probe.
type HealthResponse struct {
	Status string `json:"status"`
}

// NewsToPublicResponse maps a domain article onto its public card payload.
func NewsToPublicResponse(news *models.News) *PublicNewsResponse {
	if news == nil {
		return nil
	}

	return &PublicNewsResponse{
		Slug:        news.Slug,
		Category:    string(news.Category),
		Title:       news.Title,
		Description: news.Description,
		ImageUrl:    news.ImageUrl,
		Featured:    news.Featured,
		Views:       news.Views,
		PublishedAt: news.PublishedAt,
	}
}

// NewsToResponse maps a domain article onto its response payload.
func NewsToResponse(news *models.News) *NewsResponse {
	if news == nil {
		return nil
	}

	return &NewsResponse{
		ID:          news.ID,
		Slug:        news.Slug,
		Category:    string(news.Category),
		Title:       news.Title,
		Description: news.Description,
		Text:        news.Text,
		Language:    news.Language,
		Status:      string(news.Status),
		Views:       news.Views,
		Featured:    news.Featured,
		ImageUrl:    news.ImageUrl,
		PublishedAt: news.PublishedAt,
		CreatedAt:   news.CreatedAt,
		UpdatedAt:   news.UpdatedAt,
	}
}
