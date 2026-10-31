// Package service contains the business logic of the news platform.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
)

// Pagination limits for news lists.
const (
	DefaultPageLimit = 20
	MaxPageLimit     = 100
	// maxSlugAttempts limits suffix attempts when an auto-generated slug collides.
	maxSlugAttempts = 10
)

// NewsInput is the editable article payload accepted by the service.
type NewsInput struct {
	Category    string
	Title       string
	Description string
	Text        string
	Language    string
	Status      string
	Featured    bool
	MainEvent   bool
	ImageUrl    string
}

// ListResult is a page of articles returned by the service.
type ListResult struct {
	Items []*models.News
	Page  int
	Limit int
}

// NewsRepository is the persistence contract the service depends on.
type NewsRepository interface {
	Create(ctx context.Context, news *models.News) error
	GetBySlug(ctx context.Context, slug string) (*models.News, error)
	List(ctx context.Context, filter models.ListFilter) ([]*models.News, error)
	ListPublished(ctx context.Context, filter models.ListFilter) ([]*models.News, error)
	Update(ctx context.Context, news *models.News) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// NewsService implements the business rules for news articles.
type NewsService struct {
	repo   NewsRepository
	logger *slog.Logger
}

// NewNewsService creates a NewsService over repo.
func NewNewsService(repo NewsRepository, logger *slog.Logger) *NewsService {
	return &NewsService{repo: repo, logger: logger}
}

// Create derives a slug from the title and stores the article.
func (s *NewsService) Create(ctx context.Context, input NewsInput) (*models.News, error) {
	const fc = "newsService_Create"

	if err := validateNews(&input); err != nil {
		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	news := buildNews(input)
	baseSlug := news.Slug

	for attempt := 1; attempt <= maxSlugAttempts; attempt++ {
		candidate := *news
		if attempt > 1 {
			candidate.Slug = fmt.Sprintf("%s-%d", baseSlug, attempt)
		}

		createErr := s.repo.Create(ctx, &candidate)
		if createErr == nil {
			return &candidate, nil
		}
		if !errors.Is(createErr, errs.ErrConflict) {
			s.logger.ErrorContext(ctx, "failed to create news article", "fc", fc, "error", createErr)

			return nil, fmt.Errorf("%s: %w", fc, createErr)
		}
	}

	s.logger.ErrorContext(ctx, "failed to generate a unique slug", "fc", fc, "error", errs.ErrConflict)

	return nil, fmt.Errorf("%s: %w", fc, errs.ErrConflict)
}

// GetBySlug returns one article or errs.ErrNewsNotFound.
func (s *NewsService) GetBySlug(ctx context.Context, slug string) (*models.News, error) {
	const fc = "newsService_GetBySlug"

	news, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to get news article", "fc", fc, "error", err)

		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	return news, nil
}

// GetPublishedBySlug returns one published article or errs.ErrNewsNotFound.
func (s *NewsService) GetPublishedBySlug(ctx context.Context, slug string) (*models.News, error) {
	const fc = "newsService_GetPublishedBySlug"

	news, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to get published news article", "fc", fc, "error", err)

		return nil, fmt.Errorf("%s: %w", fc, err)
	}
	if news.Status != models.StatusPublished {
		return nil, fmt.Errorf("%s: %w", fc, errs.ErrNewsNotFound)
	}

	return news, nil
}

// List returns a page of articles, optionally filtered by status.
func (s *NewsService) List(ctx context.Context, page int, limit int, status string) (*ListResult, error) {
	const fc = "newsService_List"

	if page < 1 {
		return nil, fmt.Errorf("%s: %w: page must be at least 1", fc, errs.ErrInvalidInput)
	}
	if limit < 1 {
		return nil, fmt.Errorf("%s: %w: limit must be at least 1", fc, errs.ErrInvalidInput)
	}
	if limit > MaxPageLimit {
		limit = MaxPageLimit
	}

	filter := models.ListFilter{Page: page, Limit: limit}
	if status != "" {
		parsed := models.Status(status)
		if !parsed.Valid() {
			return nil, fmt.Errorf("%s: %w: unknown status %q", fc, errs.ErrInvalidInput, status)
		}
		filter.Status = &parsed
	}

	items, err := s.repo.List(ctx, filter)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to list news articles", "fc", fc, "error", err)

		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	return &ListResult{Items: items, Page: page, Limit: limit}, nil
}

// ListPublished returns a page of published articles for the public site.
func (s *NewsService) ListPublished(ctx context.Context, page int, limit int) (*ListResult, error) {
	const fc = "newsService_ListPublished"

	if page < 1 {
		return nil, fmt.Errorf("%s: %w: page must be at least 1", fc, errs.ErrInvalidInput)
	}
	if limit < 1 {
		return nil, fmt.Errorf("%s: %w: limit must be at least 1", fc, errs.ErrInvalidInput)
	}
	if limit > MaxPageLimit {
		limit = MaxPageLimit
	}

	items, err := s.repo.ListPublished(ctx, models.ListFilter{Page: page, Limit: limit})
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to list published news articles", "fc", fc, "error", err)

		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	return &ListResult{Items: items, Page: page, Limit: limit}, nil
}

// Update applies the editable fields of an existing article.
func (s *NewsService) Update(ctx context.Context, slug string, input NewsInput) (*models.News, error) {
	const fc = "newsService_Update"

	existing, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to load news article", "fc", fc, "error", err)

		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	inputToValidate := input
	if inputToValidate.Status == "" {
		inputToValidate.Status = string(existing.Status)
	}
	if inputToValidate.Language == "" {
		inputToValidate.Language = existing.Language
	}

	if err := validateNews(&inputToValidate); err != nil {
		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	news := buildNews(inputToValidate)

	news.ID = existing.ID
	news.Views = existing.Views
	news.CreatedAt = existing.CreatedAt
	news.PublishedAt = existing.PublishedAt
	if news.Status == models.StatusPublished && news.PublishedAt == nil {
		publishedAt := time.Now().UTC()
		news.PublishedAt = &publishedAt
	}

	if err := s.repo.Update(ctx, news); err != nil {
		s.logger.ErrorContext(ctx, "failed to update news article", "fc", fc, "error", err)

		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	return news, nil
}

// Delete removes an article by slug.
func (s *NewsService) Delete(ctx context.Context, slug string) error {
	const fc = "newsService_Delete"

	existing, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to load news article", "fc", fc, "error", err)
		return fmt.Errorf("%s: %w", fc, err)
	}

	if err := s.repo.Delete(ctx, existing.ID); err != nil {
		s.logger.ErrorContext(ctx, "failed to delete news article", "fc", fc, "error", err)
		return fmt.Errorf("%s: %w", fc, err)
	}

	return nil
}

// buildNews maps validated input onto a domain article.
func buildNews(input NewsInput) *models.News {
	status := models.StatusDraft
	if input.Status != "" {
		status = models.Status(input.Status)
	}

	now := time.Now().UTC()
	news := &models.News{
		ID:          uuid.New(),
		Slug:        slugify(input.Title),
		Category:    models.Category(input.Category),
		Title:       input.Title,
		Description: input.Description,
		Text:        input.Text,
		Language:    input.Language,
		Status:      status,
		Featured:    input.Featured,
		MainEvent:   input.MainEvent,
		ImageUrl:    input.ImageUrl,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if status == models.StatusPublished {
		news.PublishedAt = &now
	}

	return news
}
