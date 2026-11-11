package dto

import (
	"math"
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
	MainEvent   bool       `json:"main_event"`
	ImageUrl    string     `json:"image_url"`
	Source      string     `json:"source"`
	SourceURL   string     `json:"source_url"`
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
	MainEvent   bool       `json:"main_event"`
	Views       int64      `json:"views"`
	PublishedAt *time.Time `json:"published_at"`
}

// PublicNewsListResponse is a page of published article cards.
type PublicNewsListResponse struct {
	Items []*PublicNewsResponse `json:"items"`
	Page  int                   `json:"page"`
	Limit int                   `json:"limit"`
}

// PublicNewsDetailResponse is a published article returned by the public API.
type PublicNewsDetailResponse struct {
	Slug        string     `json:"slug"`
	Category    string     `json:"category"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Text        string     `json:"text"`
	ImageUrl    string     `json:"image_url"`
	Featured    bool       `json:"featured"`
	MainEvent   bool       `json:"main_event"`
	Source      string     `json:"source"`
	SourceURL   string     `json:"source_url"`
	Views       int64      `json:"views"`
	PublishedAt *time.Time `json:"published_at"`
}

// HealthResponse is the successful response of a health probe.
type HealthResponse struct {
	Status string `json:"status"`
}

// ExchangeRateResponse is one currency rate against the hryvnia.
type ExchangeRateResponse struct {
	Code string  `json:"code"`
	Rate float64 `json:"rate"`
}

// ExchangeRatesResponse is the NBU rate table served to the public site.
type ExchangeRatesResponse struct {
	Base  string                 `json:"base"`
	Date  string                 `json:"date"`
	Items []ExchangeRateResponse `json:"items"`
}

// ExchangeToRatesResponse maps domain rates onto the public payload,
// rounding each rate to two decimals in EUR, USD, PLN order.
func ExchangeToRatesResponse(rates []*models.ExchangeRate) ExchangeRatesResponse {
	items := make([]ExchangeRateResponse, 0, len(rates))

	date := ""
	for _, rate := range rates {
		if rate == nil {
			continue
		}

		if date == "" {
			date = rate.Date
		}

		items = append(items, ExchangeRateResponse{
			Code: rate.Code,
			Rate: math.Round(rate.Rate*100) / 100,
		})
	}

	return ExchangeRatesResponse{Base: "UAH", Date: date, Items: items}
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
		MainEvent:   news.MainEvent,
		Views:       news.Views,
		PublishedAt: news.PublishedAt,
	}
}

// NewsToPublicDetailResponse maps a domain article onto its public detail payload.
func NewsToPublicDetailResponse(news *models.News) *PublicNewsDetailResponse {
	if news == nil {
		return nil
	}

	return &PublicNewsDetailResponse{
		Slug:        news.Slug,
		Category:    string(news.Category),
		Title:       news.Title,
		Description: news.Description,
		Text:        news.Text,
		ImageUrl:    news.ImageUrl,
		Featured:    news.Featured,
		MainEvent:   news.MainEvent,
		Source:      news.Source,
		SourceURL:   news.SourceURL,
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
		MainEvent:   news.MainEvent,
		ImageUrl:    news.ImageUrl,
		Source:      news.Source,
		SourceURL:   news.SourceURL,
		PublishedAt: news.PublishedAt,
		CreatedAt:   news.CreatedAt,
		UpdatedAt:   news.UpdatedAt,
	}
}
