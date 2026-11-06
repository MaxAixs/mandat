// Package service contains the business logic of the news platform.
package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

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

// Limits for published-news title search.
const (
	DefaultSearchLimit = 7
	MaxSearchLimit     = 20
	MaxSearchQueryLen  = 20
)

// NewsRepository is the persistence contract the service depends on.
type NewsRepository interface {
	Create(ctx context.Context, news *models.News) error
	GetBySlug(ctx context.Context, slug string) (*models.News, error)
	List(ctx context.Context, filter models.ListFilter) ([]*models.News, error)
	ListPublished(ctx context.Context, filter models.ListFilter) ([]*models.News, error)
	SearchPublished(ctx context.Context, query string, limit int) ([]*models.News, error)
	Update(ctx context.Context, news *models.News) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// ImageStorage uploads and deletes cover images.
// The service owns this contract; pkg/storage/r2 implements it.
type ImageStorage interface {
	Upload(ctx context.Context, key string, body io.Reader, contentType string) (string, error)
	Delete(ctx context.Context, key string) error
}

// ImageFetcher downloads pasted image URLs for cover storage.
// The service owns this contract; internal/imgdownloader implements it.
type ImageFetcher interface {
	Fetch(ctx context.Context, url string) ([]byte, string, error)
}

// NewsService implements the business rules for news articles.
type NewsService struct {
	repo         NewsRepository
	logger       *slog.Logger
	storage      ImageStorage
	coverBaseURL string
	fetcher      ImageFetcher
}

// NewNewsService creates a fully initialized NewsService.
// Storage and fetcher are required: main fails startup when R2 is down,
// so cover paths never meet a nil dependency here. A nil fetcher alone
// keeps the legacy behavior of storing pasted URLs as-is.
func NewNewsService(
	repo NewsRepository,
	logger *slog.Logger,
	storage ImageStorage,
	publicBaseURL string,
	fetcher ImageFetcher,
) *NewsService {
	return &NewsService{repo: repo, logger: logger, storage: storage, coverBaseURL: publicBaseURL, fetcher: fetcher}
}

// NewsInput is the editable article payload accepted by the service.
// Category and Status are domain enums validated by the service.
type NewsInput struct {
	Category    models.Category
	Title       string
	Description string
	Text        string
	Language    string
	Status      models.Status
	Featured    bool
	MainEvent   bool
	ImageUrl    string
	Source      string
	SourceURL   string
	// Cover, when non-nil, uploads a new cover image and replaces ImageUrl.
	Cover *CoverUpload
}

// CoverUpload is a raw cover file supplied with a create or update call.
type CoverUpload struct {
	Body        io.Reader
	Filename    string
	ContentType string
	Size        int64
}

// ListResult is a page of articles returned by the service.
type ListResult struct {
	Items []*models.News
	Page  int
	Limit int
}

// coverExtWhitelist lists the filename extensions accepted for covers.
// Content itself is re-validated by the storage driver.
var coverExtWhitelist = map[string]struct{}{
	"jpg":  {},
	"jpeg": {},
	"png":  {},
	"webp": {},
	"gif":  {},
}

// fetchedExt maps sniffed image types onto cover key extensions.
var fetchedExt = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/webp": "webp",
	"image/gif":  "gif",
}

// Create derives a slug from the title and stores the article.
func (s *NewsService) Create(ctx context.Context, input NewsInput) (*models.News, error) {
	const fc = "newsService_Create"

	if err := validateNews(&input); err != nil {
		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	cover, imageURL, err := s.resolvePastedURL(ctx, input.ImageUrl, input.Cover)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to fetch pasted image", "fc", fc, "error", err)

		return nil, fmt.Errorf("%s: %w", fc, err)
	}
	input.Cover, input.ImageUrl = cover, imageURL

	imgExt := ""
	if input.Cover != nil {
		ext, err := coverExt(input.Cover.Filename)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", fc, err)
		}
		imgExt = ext
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
			return s.storeImg(ctx, &candidate, input.Cover, imgExt)
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

// List returns a page of articles, optionally filtered by status and category.
func (s *NewsService) List(ctx context.Context, page int, limit int, status string, category string) (*ListResult, error) {
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
	if category != "" {
		parsed := models.Category(category)
		if !parsed.Valid() {
			return nil, fmt.Errorf("%s: %w: unknown category %q", fc, errs.ErrInvalidInput, category)
		}
		filter.Category = &parsed
	}

	items, err := s.repo.List(ctx, filter)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to list news articles", "fc", fc, "error", err)

		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	return &ListResult{Items: items, Page: page, Limit: limit}, nil
}

// ListPublished returns a page of published articles for the public site.
func (s *NewsService) ListPublished(ctx context.Context, page int, limit int, category string) (*ListResult, error) {
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

	filter := models.ListFilter{Page: page, Limit: limit}
	if category != "" {
		parsed := models.Category(category)
		if !parsed.Valid() {
			return nil, fmt.Errorf("%s: %w: unknown category %q", fc, errs.ErrInvalidInput, category)
		}
		filter.Category = &parsed
	}

	items, err := s.repo.ListPublished(ctx, filter)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to list published news articles", "fc", fc, "error", err)

		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	return &ListResult{Items: items, Page: page, Limit: limit}, nil
}

// SearchPublished returns up to limit published articles whose title contains query.
func (s *NewsService) SearchPublished(ctx context.Context, query string, limit int) (*ListResult, error) {
	const fc = "newsService_SearchPublished"

	if limit < 1 {
		return nil, fmt.Errorf("%s: %w: limit must be at least 1", fc, errs.ErrInvalidInput)
	}
	resolvedLimit := min(limit, MaxSearchLimit)

	trimmedQuery := strings.TrimSpace(query)
	if trimmedQuery == "" {
		return &ListResult{Items: []*models.News{}, Page: 1, Limit: resolvedLimit}, nil
	}
	if !utf8.ValidString(trimmedQuery) || strings.ContainsRune(trimmedQuery, 0) {
		return nil, fmt.Errorf("%s: %w: malformed query", fc, errs.ErrInvalidInput)
	}
	if utf8.RuneCountInString(trimmedQuery) > MaxSearchQueryLen {
		return nil, fmt.Errorf("%s: %w: query too long", fc, errs.ErrInvalidInput)
	}

	items, err := s.repo.SearchPublished(ctx, trimmedQuery, resolvedLimit)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to search published news articles", "fc", fc, "error", err)

		return nil, fmt.Errorf("%s: %w", fc, err)
	}
	if len(items) == 0 {
		items = []*models.News{}
	}

	return &ListResult{Items: items, Page: 1, Limit: resolvedLimit}, nil
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
		inputToValidate.Status = existing.Status
	}
	if inputToValidate.Language == "" {
		inputToValidate.Language = existing.Language
	}

	if err := validateNews(&inputToValidate); err != nil {
		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	cover, imageURL, err := s.resolvePastedURL(ctx, inputToValidate.ImageUrl, inputToValidate.Cover)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fc, err)
	}
	inputToValidate.Cover, inputToValidate.ImageUrl = cover, imageURL
	input = inputToValidate

	covExt := ""
	if input.Cover != nil {
		ext, err := coverExt(input.Cover.Filename)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", fc, err)
		}
		covExt = ext
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
	if input.Cover == nil && input.ImageUrl == "" {
		news.ImageUrl = existing.ImageUrl
	}

	if input.Cover == nil {
		if err := s.repo.Update(ctx, news); err != nil {
			s.logger.ErrorContext(ctx, "failed to update news article", "fc", fc, "error", err)

			return nil, fmt.Errorf("%s: %w", fc, err)
		}

		return news, nil
	}

	key := "covers/" + news.Slug + "." + covExt
	url, err := s.storage.Upload(ctx, key, input.Cover.Body, input.Cover.ContentType)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to upload news cover", "fc", fc, "error", err)

		return nil, fmt.Errorf("%s: %w", fc, err)
	}
	news.ImageUrl = url

	if err := s.repo.Update(ctx, news); err != nil {
		s.logger.ErrorContext(ctx, "failed to update news article", "fc", fc, "error", err)

		if derr := s.storage.Delete(ctx, key); derr != nil {
			s.logger.ErrorContext(ctx, "failed to delete orphaned news cover", "fc", fc, "key", key, "error", derr)
		}

		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	if oldKey, _ := s.ownedCoverKey(existing.ImageUrl); oldKey != "" && oldKey != key {
		if err := s.storage.Delete(ctx, oldKey); err != nil {
			s.logger.ErrorContext(ctx, "failed to delete replaced news cover", "fc", fc, "key", oldKey, "error", err)
		}
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

	if key, owned := s.ownedCoverKey(existing.ImageUrl); owned {
		if err := s.storage.Delete(ctx, key); err != nil {
			s.logger.ErrorContext(ctx, "failed to delete news cover", "fc", fc, "key", key, "error", err)
		}
	}

	return nil
}

// resolvePastedURL downloads an external image URL into a cover upload.
// It runs before any database write, so failures leave no partial state.
// The returned cover and image URL replace the passed ones.
func (s *NewsService) resolvePastedURL(ctx context.Context, imageURL string, cover *CoverUpload) (*CoverUpload, string, error) {
	if cover != nil || s.fetcher == nil || !s.isExternalImageURL(imageURL) {
		return cover, imageURL, nil
	}

	data, mime, err := s.fetcher.Fetch(ctx, imageURL)
	if err != nil {
		return nil, "", err
	}

	imgExt, ok := fetchedExt[mime]
	if !ok {
		return nil, "", errs.NewValidationError("image_url", "unsupported image type "+mime)
	}

	return &CoverUpload{Body: bytes.NewReader(data), Filename: "image." + imgExt, ContentType: mime}, "", nil
}

// isExternalImageURL reports pasted links pointing outside our bucket.
func (s *NewsService) isExternalImageURL(raw string) bool {
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		return false
	}
	_, owned := s.ownedCoverKey(raw)

	return !owned
}

// storeImg uploads cover for a freshly created article and persists its
// URL. A nil cover leaves the article as is. Failures compensate the
// created row (and the uploaded object) so no orphan state survives.
func (s *NewsService) storeImg(ctx context.Context, news *models.News, cover *CoverUpload, ext string) (*models.News, error) {
	if cover == nil {
		return news, nil
	}

	key := "covers/" + news.Slug + "." + ext
	prevURL := news.ImageUrl
	url, err := s.storage.Upload(ctx, key, cover.Body, cover.ContentType)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to upload news cover, article kept without cover", "error", err)

		return news, nil
	}

	news.ImageUrl = url
	if err := s.repo.Update(ctx, news); err != nil {
		s.logger.ErrorContext(ctx, "failed to save news cover url", "error", err)
		if derr := s.storage.Delete(ctx, key); derr != nil {
			s.logger.ErrorContext(ctx, "failed to delete orphaned news cover", "key", key, "error", derr)
		}
		news.ImageUrl = prevURL

		return news, nil
	}

	return news, nil
}


// buildNews maps validated input onto a domain article.
func buildNews(input NewsInput) *models.News {
	status := models.StatusDraft
	if input.Status != "" {
		status = input.Status
	}

	now := time.Now().UTC()
	news := &models.News{
		ID:          uuid.New(),
		Slug:        slugify(input.Title),
		Category:    input.Category,
		Title:       input.Title,
		Description: input.Description,
		Text:        input.Text,
		Language:    input.Language,
		Status:      status,
		Featured:    input.Featured,
		MainEvent:   input.MainEvent,
		ImageUrl:    input.ImageUrl,
		Source:      input.Source,
		SourceURL:   input.SourceURL,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if status == models.StatusPublished {
		news.PublishedAt = &now
	}

	return news
}

// coverExt validates the cover filename and returns its lowercase extension.
func coverExt(filename string) (string, error) {
	dotIndex := strings.LastIndex(filename, ".")
	if dotIndex < 0 || dotIndex == len(filename)-1 {
		return "", errs.NewValidationError("cover", "file must have an image extension")
	}
	ext := strings.ToLower(filename[dotIndex+1:])
	if _, ok := coverExtWhitelist[ext]; !ok {
		return "", errs.NewValidationError("cover", "unsupported image type "+ext)
	}

	return ext, nil
}

// ownedCoverKey extracts the storage key when url points at our bucket.
// Only keys under covers/ qualify: shared assets on the same host stay
// untouched, and unsafe key shapes are rejected.
func (s *NewsService) ownedCoverKey(url string) (string, bool) {
	if s.storage == nil || s.coverBaseURL == "" || url == "" {
		return "", false
	}

	prefix := strings.TrimRight(s.coverBaseURL, "/") + "/"
	if !strings.HasPrefix(url, prefix) {
		return "", false
	}
	key := strings.TrimPrefix(url, prefix)
	if !strings.HasPrefix(key, "covers/") || strings.Contains(key, "..") || strings.HasPrefix(key, "/") {
		return "", false
	}

	return key, true
}
