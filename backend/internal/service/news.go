// Package service contains the business logic of the news platform.
package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
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
	"github.com/maxaixs/mandat/backend/pkg/imagetype"
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

// MaxCoverBytes caps one cover image, uploaded or fetched from a pasted URL.
const MaxCoverBytes = 10 << 20

// Cover object keys: covers/<slug>-<random hex>.<ext>. The random part makes every
// upload a new object, so replacing a cover never overwrites the live one.
const (
	coverKeyPrefix      = "covers/"
	coverKeySuffixBytes = 4
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
// The service owns this contract; internal/storage/r2 implements it.
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

// NewNewsService creates a NewsService. Storage is required for cover uploads;
// a nil fetcher stores pasted image URLs as-is instead of downloading them.
func NewNewsService(
	repo NewsRepository,
	logger *slog.Logger,
	storage ImageStorage,
	publicBaseURL string,
	fetcher ImageFetcher,
) *NewsService {
	return &NewsService{
		repo:         repo,
		logger:       logger,
		storage:      storage,
		coverBaseURL: publicBaseURL,
		fetcher:      fetcher,
	}
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
	ImageURL    string
	Source      string
	SourceURL   string
	// Cover, when non-nil, uploads a new cover image and replaces ImageURL.
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
// HasMore reports whether a next page exists.
type ListResult struct {
	Items   []*models.News
	Page    int
	Limit   int
	HasMore bool
}

// coverImage is a cover whose bytes were read and checked before any write.
type coverImage struct {
	data        []byte
	ext         string
	contentType string
}

// Create derives a slug from the title and stores the article.
// The cover is validated before the row is written; if storing it fails
// afterwards, the row is removed so no half-created article survives.
func (s *NewsService) Create(ctx context.Context, input NewsInput) (*models.News, error) {
	const fc = "newsService_Create"

	if err := validateNews(&input); err != nil {
		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	cover, err := s.prepareCover(ctx, &input)
	if err != nil {
		s.logFailure(ctx, "failed to prepare news cover", fc, err)

		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	news, err := s.insertWithUniqueSlug(ctx, buildNews(input))
	if err != nil {
		s.logFailure(ctx, "failed to create news article", fc, err)

		return nil, fmt.Errorf("%s: %w", fc, err)
	}
	if cover == nil {
		return news, nil
	}

	if err := s.attachCover(ctx, news, cover); err != nil {
		s.logFailure(ctx, "failed to store news cover, article removed", fc, err)
		if derr := s.repo.Delete(ctx, news.ID); derr != nil {
			s.logger.ErrorContext(ctx, "failed to remove half-created news article", "fc", fc, "id", news.ID, "error", derr)
		}

		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	return news, nil
}

// GetBySlug returns one article or errs.ErrNewsNotFound.
func (s *NewsService) GetBySlug(ctx context.Context, slug string) (*models.News, error) {
	const fc = "newsService_GetBySlug"

	news, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		s.logFailure(ctx, "failed to get news article", fc, err)

		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	return news, nil
}

// GetPublishedBySlug returns one published article or errs.ErrNewsNotFound.
func (s *NewsService) GetPublishedBySlug(ctx context.Context, slug string) (*models.News, error) {
	const fc = "newsService_GetPublishedBySlug"

	news, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		s.logFailure(ctx, "failed to get published news article", fc, err)

		return nil, fmt.Errorf("%s: %w", fc, err)
	}
	if news.Status != models.StatusPublished {
		return nil, fmt.Errorf("%s: %w", fc, errs.ErrNewsNotFound)
	}

	return news, nil
}

// List returns a page of articles, optionally filtered by status and category.
func (s *NewsService) List(
	ctx context.Context,
	page int,
	limit int,
	status string,
	category string,
) (*ListResult, error) {
	const fc = "newsService_List"

	limit, err := normalizePage(page, limit)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	// One extra row reveals whether a next page exists.
	filter := models.ListFilter{Limit: limit + 1, Offset: (page - 1) * limit}
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
		s.logFailure(ctx, "failed to list news articles", fc, err)

		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}

	return &ListResult{Items: items, Page: page, Limit: limit, HasMore: hasMore}, nil
}

// ListPublished returns a page of published articles for the public site.
func (s *NewsService) ListPublished(ctx context.Context, page int, limit int, category string) (*ListResult, error) {
	const fc = "newsService_ListPublished"

	limit, err := normalizePage(page, limit)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	// One extra row reveals whether a next page exists.
	filter := models.ListFilter{Limit: limit + 1, Offset: (page - 1) * limit}
	if category != "" {
		parsed := models.Category(category)
		if !parsed.Valid() {
			return nil, fmt.Errorf("%s: %w: unknown category %q", fc, errs.ErrInvalidInput, category)
		}
		filter.Category = &parsed
	}

	items, err := s.repo.ListPublished(ctx, filter)
	if err != nil {
		s.logFailure(ctx, "failed to list published news articles", fc, err)

		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}

	return &ListResult{Items: items, Page: page, Limit: limit, HasMore: hasMore}, nil
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
		s.logFailure(ctx, "failed to search published news articles", fc, err)

		return nil, fmt.Errorf("%s: %w", fc, err)
	}
	if len(items) == 0 {
		items = []*models.News{}
	}

	return &ListResult{Items: items, Page: 1, Limit: resolvedLimit}, nil
}

// Update applies the editable fields of an existing article.
// The slug is fixed at creation and never changes, so links stay valid.
func (s *NewsService) Update(ctx context.Context, slug string, input NewsInput) (*models.News, error) {
	const fc = "newsService_Update"

	existing, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		s.logFailure(ctx, "failed to load news article", fc, err)

		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	if input.Status == "" {
		input.Status = existing.Status
	}
	if input.Language == "" {
		input.Language = existing.Language
	}
	if err := validateNews(&input); err != nil {
		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	cover, err := s.prepareCover(ctx, &input)
	if err != nil {
		s.logFailure(ctx, "failed to prepare news cover", fc, err)

		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	news := buildUpdated(input, existing)
	if cover == nil {
		if err := s.repo.Update(ctx, news); err != nil {
			s.logFailure(ctx, "failed to update news article", fc, err)

			return nil, fmt.Errorf("%s: %w", fc, err)
		}

		return news, nil
	}

	if err := s.replaceCover(ctx, news, existing.ImageURL, cover); err != nil {
		s.logFailure(ctx, "failed to update news article cover", fc, err)

		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	return news, nil
}

// Delete removes an article by slug together with its owned cover.
func (s *NewsService) Delete(ctx context.Context, slug string) error {
	const fc = "newsService_Delete"

	existing, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		s.logFailure(ctx, "failed to load news article", fc, err)

		return fmt.Errorf("%s: %w", fc, err)
	}

	if err := s.repo.Delete(ctx, existing.ID); err != nil {
		s.logFailure(ctx, "failed to delete news article", fc, err)

		return fmt.Errorf("%s: %w", fc, err)
	}

	if key, owned := s.ownedCoverKey(existing.ImageURL); owned {
		if err := s.storage.Delete(ctx, key); err != nil {
			s.logger.ErrorContext(ctx, "failed to delete news cover", "fc", fc, "key", key, "error", err)
		}
	}

	return nil
}

// prepareCover reads and validates the cover before any write. A pasted
// external image URL is downloaded and becomes the cover. Nil means no cover.
func (s *NewsService) prepareCover(ctx context.Context, input *NewsInput) (*coverImage, error) {
	const fc = "newsService_prepareCover"

	var (
		cover *coverImage
		err   error
	)
	switch {
	case input.Cover != nil:
		cover, err = readCover(input.Cover)
	case s.fetcher != nil && s.isExternalImageURL(input.ImageURL):
		cover, err = s.fetchCover(ctx, input.ImageURL)
		input.ImageURL = ""
	default:
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fc, err)
	}
	if s.storage == nil {
		return nil, fmt.Errorf("%s: %w: cover storage is not configured", fc, errs.ErrServiceUnavailable)
	}

	return cover, nil
}

// fetchCover downloads a pasted image URL and checks it is a supported image.
func (s *NewsService) fetchCover(ctx context.Context, url string) (*coverImage, error) {
	const fc = "newsService_fetchCover"

	data, mime, err := s.fetcher.Fetch(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fc, err)
	}
	if len(data) > MaxCoverBytes {
		return nil, fmt.Errorf("%s: %w: image exceeds %d bytes", fc, errs.ErrPayloadTooLarge, MaxCoverBytes)
	}

	ext, ok := imagetype.ExtForMIME(mime)
	if !ok {
		return nil, fmt.Errorf("%s: %w", fc, errs.NewValidationError("image_url", "unsupported image type "+mime))
	}

	return &coverImage{data: data, ext: ext, contentType: mime}, nil
}

// insertWithUniqueSlug stores news, suffixing the slug (-2, -3, ...) on collisions.
func (s *NewsService) insertWithUniqueSlug(ctx context.Context, news *models.News) (*models.News, error) {
	const fc = "newsService_insertWithUniqueSlug"

	baseSlug := news.Slug
	for attempt := 1; attempt <= maxSlugAttempts; attempt++ {
		candidate := *news
		if attempt > 1 {
			candidate.Slug = fmt.Sprintf("%s-%d", baseSlug, attempt)
		}

		err := s.repo.Create(ctx, &candidate)
		if err == nil {
			return &candidate, nil
		}
		if !errors.Is(err, errs.ErrConflict) {
			return nil, fmt.Errorf("%s: %w", fc, err)
		}
	}

	return nil, fmt.Errorf("%s: no free slug for %q: %w", fc, baseSlug, errs.ErrConflict)
}

// attachCover uploads the cover of a freshly created article and saves its URL.
// A failed save removes the uploaded object again.
func (s *NewsService) attachCover(ctx context.Context, news *models.News, cover *coverImage) error {
	const fc = "newsService_attachCover"

	key, url, err := s.uploadCover(ctx, news.Slug, cover)
	if err != nil {
		return fmt.Errorf("%s: %w", fc, err)
	}

	news.ImageURL = url
	if err := s.repo.Update(ctx, news); err != nil {
		if derr := s.storage.Delete(ctx, key); derr != nil {
			s.logger.ErrorContext(ctx, "failed to delete orphaned news cover", "fc", fc, "key", key, "error", derr)
		}

		return fmt.Errorf("%s: save cover url: %w", fc, err)
	}

	return nil
}

// replaceCover uploads a new cover, saves the article, and only then deletes
// the previous owned cover. A failed save deletes the new object only.
func (s *NewsService) replaceCover(ctx context.Context, news *models.News, oldURL string, cover *coverImage) error {
	const fc = "newsService_replaceCover"

	key, url, err := s.uploadCover(ctx, news.Slug, cover)
	if err != nil {
		return fmt.Errorf("%s: %w", fc, err)
	}

	news.ImageURL = url
	if err := s.repo.Update(ctx, news); err != nil {
		if derr := s.storage.Delete(ctx, key); derr != nil {
			s.logger.ErrorContext(ctx, "failed to delete orphaned news cover", "fc", fc, "key", key, "error", derr)
		}

		return fmt.Errorf("%s: %w", fc, err)
	}

	if oldKey, owned := s.ownedCoverKey(oldURL); owned && oldKey != key {
		if err := s.storage.Delete(ctx, oldKey); err != nil {
			s.logger.ErrorContext(ctx, "failed to delete replaced news cover", "fc", fc, "key", oldKey, "error", err)
		}
	}

	return nil
}

// uploadCover stores cover under a fresh key and returns the key and its public URL.
// Storage failures other than rejected content are reported as unavailable.
func (s *NewsService) uploadCover(ctx context.Context, slug string, cover *coverImage) (string, string, error) {
	const fc = "newsService_uploadCover"

	key := coverKey(slug, cover.ext)
	url, err := s.storage.Upload(ctx, key, bytes.NewReader(cover.data), cover.contentType)
	if err != nil {
		if !errors.Is(err, errs.ErrInvalidInput) && !errors.Is(err, errs.ErrPayloadTooLarge) {
			err = fmt.Errorf("%w: %w", errs.ErrServiceUnavailable, err)
		}

		return "", "", fmt.Errorf("%s: upload %s: %w", fc, key, err)
	}

	return key, url, nil
}

// isExternalImageURL reports pasted links pointing outside our bucket.
func (s *NewsService) isExternalImageURL(raw string) bool {
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		return false
	}
	_, owned := s.ownedCoverKey(raw)

	return !owned
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
	if !strings.HasPrefix(key, coverKeyPrefix) || strings.Contains(key, "..") || strings.HasPrefix(key, "/") {
		return "", false
	}

	return key, true
}

// logFailure logs unexpected failures with fc. Expected outcomes (missing
// article, invalid input, conflicts) are the client's problem and stay quiet.
func (s *NewsService) logFailure(ctx context.Context, msg string, fc string, err error) {
	if errors.Is(err, errs.ErrNewsNotFound) ||
		errors.Is(err, errs.ErrInvalidInput) ||
		errors.Is(err, errs.ErrPayloadTooLarge) ||
		errors.Is(err, errs.ErrConflict) {
		return
	}

	s.logger.ErrorContext(ctx, msg, "fc", fc, "error", err)
}

// normalizePage validates page and limit and caps limit at MaxPageLimit.
func normalizePage(page int, limit int) (int, error) {
	if page < 1 {
		return 0, fmt.Errorf("%w: page must be at least 1", errs.ErrInvalidInput)
	}
	if limit < 1 {
		return 0, fmt.Errorf("%w: limit must be at least 1", errs.ErrInvalidInput)
	}

	return min(limit, MaxPageLimit), nil
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
		ImageURL:    input.ImageURL,
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

// buildUpdated maps validated input onto existing, keeping the system-owned
// fields: ID, slug, views, creation and first publication time.
func buildUpdated(input NewsInput, existing *models.News) *models.News {
	news := buildNews(input)

	news.ID = existing.ID
	news.Slug = existing.Slug
	news.Views = existing.Views
	news.CreatedAt = existing.CreatedAt
	news.PublishedAt = existing.PublishedAt
	if news.Status == models.StatusPublished && news.PublishedAt == nil {
		publishedAt := news.UpdatedAt
		news.PublishedAt = &publishedAt
	}
	if input.ImageURL == "" {
		news.ImageURL = existing.ImageURL
	}

	return news
}

// readCover reads an uploaded cover and checks that its content is an image
// matching the filename extension; the client's MIME type is not trusted.
func readCover(upload *CoverUpload) (*coverImage, error) {
	ext, err := coverExt(upload.Filename)
	if err != nil {
		return nil, err
	}
	if upload.Body == nil {
		return nil, errs.NewValidationError("cover", "file is empty")
	}

	data, err := io.ReadAll(io.LimitReader(upload.Body, MaxCoverBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read cover: %w", err)
	}
	if len(data) > MaxCoverBytes {
		return nil, fmt.Errorf("%w: cover exceeds %d bytes", errs.ErrPayloadTooLarge, MaxCoverBytes)
	}

	sniffed := imagetype.Sniff(data)
	want, _ := imagetype.MIMEForExt(ext)
	if sniffed != want {
		return nil, errs.NewValidationError("cover", "file extension "+ext+" does not match content "+sniffed)
	}

	return &coverImage{data: data, ext: ext, contentType: sniffed}, nil
}

// coverExt validates the cover filename and returns its lowercase extension.
func coverExt(filename string) (string, error) {
	dotIndex := strings.LastIndex(filename, ".")
	if dotIndex < 0 || dotIndex == len(filename)-1 {
		return "", errs.NewValidationError("cover", "file must have an image extension")
	}
	ext := strings.ToLower(filename[dotIndex+1:])
	if _, ok := imagetype.MIMEForExt(ext); !ok {
		return "", errs.NewValidationError("cover", "unsupported image type "+ext)
	}

	return ext, nil
}

// coverKey names the storage object of a new cover upload: unique per upload.
func coverKey(slug string, ext string) string {
	suffix := make([]byte, coverKeySuffixBytes)
	_, _ = rand.Read(suffix) // crypto/rand.Read never fails; it aborts the program instead.

	return coverKeyPrefix + slug + "-" + hex.EncodeToString(suffix) + "." + ext
}
