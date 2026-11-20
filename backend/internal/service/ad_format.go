package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
)

// Limits of an ad format card, mirrored by the ad_formats table checks.
const (
	AdFormatTitleMaxLen       = 80
	AdFormatDescriptionMaxLen = 240
)

// Move directions accepted by AdFormatService.Move.
const (
	MoveUp   = "up"
	MoveDown = "down"
)

// Ad format image keys: ad-formats/<id>-<random hex>.<ext>, a new object per upload.
const (
	adFormatKeyPrefix      = "ad-formats/"
	adFormatKeySuffixBytes = 4
)

// AdFormatRepository is the persistence contract of ad format cards.
type AdFormatRepository interface {
	List(ctx context.Context) ([]*models.AdFormat, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.AdFormat, error)
	// Create appends the card after the last one and sets its Position.
	Create(ctx context.Context, format *models.AdFormat) error
	Update(ctx context.Context, format *models.AdFormat) error
	Delete(ctx context.Context, id uuid.UUID) error
	// Move swaps the card with its neighbour above (up) or below; no neighbour is a no-op.
	Move(ctx context.Context, id uuid.UUID, up bool) error
}

// AdFormatInput is the editable payload of an ad format card.
// Image, when non-nil, replaces the current image; RemoveImage clears it.
type AdFormatInput struct {
	Title       string
	Description string
	Image       *CoverUpload
	RemoveImage bool
}

// AdFormatService implements the business rules of ad format cards.
type AdFormatService struct {
	repo          AdFormatRepository
	logger        *slog.Logger
	storage       ImageStorage
	publicBaseURL string
}

// NewAdFormatService creates an AdFormatService. Storage is required for image uploads.
func NewAdFormatService(
	repo AdFormatRepository,
	logger *slog.Logger,
	storage ImageStorage,
	publicBaseURL string,
) *AdFormatService {
	return &AdFormatService{repo: repo, logger: logger, storage: storage, publicBaseURL: publicBaseURL}
}

// List returns all cards ordered by position.
func (s *AdFormatService) List(ctx context.Context) ([]*models.AdFormat, error) {
	const fc = "adFormatService_List"

	formats, err := s.repo.List(ctx)
	if err != nil {
		s.logFailure(ctx, "failed to list ad formats", fc, err)

		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	return formats, nil
}

// Create validates and stores a new card at the end of the list. The image is
// checked before the row is written; if storing it fails, the row is removed.
func (s *AdFormatService) Create(ctx context.Context, input AdFormatInput) (*models.AdFormat, error) {
	const fc = "adFormatService_Create"

	if err := validateAdFormat(&input); err != nil {
		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	image, err := s.prepareImage(input.Image)
	if err != nil {
		s.logFailure(ctx, "failed to prepare ad format image", fc, err)

		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	now := time.Now().UTC()
	format := &models.AdFormat{
		ID:          uuid.New(),
		Title:       input.Title,
		Description: input.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.repo.Create(ctx, format); err != nil {
		s.logFailure(ctx, "failed to create ad format", fc, err)

		return nil, fmt.Errorf("%s: %w", fc, err)
	}
	if image == nil {
		return format, nil
	}

	if err := s.saveImage(ctx, format, image); err != nil {
		s.logFailure(ctx, "failed to store ad format image, card removed", fc, err)
		if derr := s.repo.Delete(ctx, format.ID); derr != nil {
			s.logger.ErrorContext(ctx, "failed to remove half-created ad format", "fc", fc, "id", format.ID, "error", derr)
		}

		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	return format, nil
}

// Update changes the card text and replaces or removes its image. The previous
// owned image is deleted only after the new state is saved.
func (s *AdFormatService) Update(ctx context.Context, id uuid.UUID, input AdFormatInput) (*models.AdFormat, error) {
	const fc = "adFormatService_Update"

	if err := validateAdFormat(&input); err != nil {
		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	image, err := s.prepareImage(input.Image)
	if err != nil {
		s.logFailure(ctx, "failed to prepare ad format image", fc, err)

		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.logFailure(ctx, "failed to load ad format", fc, err)

		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	format := *existing
	format.Title = input.Title
	format.Description = input.Description
	format.UpdatedAt = time.Now().UTC()

	switch {
	case image != nil:
		err = s.saveImage(ctx, &format, image)
	case input.RemoveImage:
		format.ImageURL = ""
		err = s.repo.Update(ctx, &format)
	default:
		err = s.repo.Update(ctx, &format)
	}
	if err != nil {
		s.logFailure(ctx, "failed to update ad format", fc, err)

		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	if format.ImageURL != existing.ImageURL {
		s.deleteOwnedImage(ctx, fc, existing.ImageURL)
	}

	return &format, nil
}

// Delete removes the card together with its stored image.
func (s *AdFormatService) Delete(ctx context.Context, id uuid.UUID) error {
	const fc = "adFormatService_Delete"

	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.logFailure(ctx, "failed to load ad format", fc, err)

		return fmt.Errorf("%s: %w", fc, err)
	}

	if err := s.repo.Delete(ctx, existing.ID); err != nil {
		s.logFailure(ctx, "failed to delete ad format", fc, err)

		return fmt.Errorf("%s: %w", fc, err)
	}

	s.deleteOwnedImage(ctx, fc, existing.ImageURL)

	return nil
}

// Move shifts the card one place up or down.
func (s *AdFormatService) Move(ctx context.Context, id uuid.UUID, direction string) error {
	const fc = "adFormatService_Move"

	if direction != MoveUp && direction != MoveDown {
		return fmt.Errorf("%s: %w", fc, errs.NewValidationError("direction", "must be up or down"))
	}

	if err := s.repo.Move(ctx, id, direction == MoveUp); err != nil {
		s.logFailure(ctx, "failed to move ad format", fc, err)

		return fmt.Errorf("%s: %w", fc, err)
	}

	return nil
}

// prepareImage reads and validates an uploaded image before any write. Nil means no image.
func (s *AdFormatService) prepareImage(upload *CoverUpload) (*coverImage, error) {
	const fc = "adFormatService_prepareImage"

	if upload == nil {
		return nil, nil
	}

	image, err := readCover(upload)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fc, err)
	}
	if s.storage == nil {
		return nil, fmt.Errorf("%s: %w: image storage is not configured", fc, errs.ErrServiceUnavailable)
	}

	return image, nil
}

// saveImage uploads the image under a fresh key and saves the card with its URL.
// A failed save deletes the uploaded object again.
func (s *AdFormatService) saveImage(ctx context.Context, format *models.AdFormat, image *coverImage) error {
	const fc = "adFormatService_saveImage"

	key := adFormatImageKey(format.ID, image.ext)
	url, err := s.storage.Upload(ctx, key, bytes.NewReader(image.data), image.contentType)
	if err != nil {
		if !errors.Is(err, errs.ErrInvalidInput) && !errors.Is(err, errs.ErrPayloadTooLarge) {
			err = fmt.Errorf("%w: %w", errs.ErrServiceUnavailable, err)
		}

		return fmt.Errorf("%s: upload %s: %w", fc, key, err)
	}

	format.ImageURL = url
	if err := s.repo.Update(ctx, format); err != nil {
		if derr := s.storage.Delete(ctx, key); derr != nil {
			s.logger.ErrorContext(ctx, "failed to delete orphaned ad format image", "fc", fc, "key", key, "error", derr)
		}

		return fmt.Errorf("%s: save image url: %w", fc, err)
	}

	return nil
}

// deleteOwnedImage removes url from storage when it is one of our ad format
// objects; failures are only logged, the card change already succeeded.
func (s *AdFormatService) deleteOwnedImage(ctx context.Context, fc string, url string) {
	key, owned := s.ownedImageKey(url)
	if !owned {
		return
	}
	if err := s.storage.Delete(ctx, key); err != nil {
		s.logger.ErrorContext(ctx, "failed to delete ad format image", "fc", fc, "key", key, "error", err)
	}
}

// ownedImageKey extracts the storage key when url points at an ad format
// object in our bucket; other paths and unsafe key shapes are rejected.
func (s *AdFormatService) ownedImageKey(url string) (string, bool) {
	if s.storage == nil || s.publicBaseURL == "" || url == "" {
		return "", false
	}

	prefix := strings.TrimRight(s.publicBaseURL, "/") + "/"
	if !strings.HasPrefix(url, prefix) {
		return "", false
	}
	key := strings.TrimPrefix(url, prefix)
	if !strings.HasPrefix(key, adFormatKeyPrefix) || strings.Contains(key, "..") {
		return "", false
	}

	return key, true
}

// logFailure logs unexpected failures with fc; client errors stay quiet.
func (s *AdFormatService) logFailure(ctx context.Context, msg string, fc string, err error) {
	if errors.Is(err, errs.ErrAdFormatNotFound) ||
		errors.Is(err, errs.ErrInvalidInput) ||
		errors.Is(err, errs.ErrPayloadTooLarge) ||
		errors.Is(err, errs.ErrConflict) {
		return
	}

	s.logger.ErrorContext(ctx, msg, "fc", fc, "error", err)
}

// validateAdFormat trims the text fields and checks their lengths in runes.
func validateAdFormat(input *AdFormatInput) error {
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)

	if input.Title == "" {
		return errs.NewValidationError("title", "title is required")
	}
	if utf8.RuneCountInString(input.Title) > AdFormatTitleMaxLen {
		return errs.NewValidationError("title", fmt.Sprintf("title must be at most %d characters", AdFormatTitleMaxLen))
	}
	if utf8.RuneCountInString(input.Description) > AdFormatDescriptionMaxLen {
		return errs.NewValidationError(
			"description",
			fmt.Sprintf("description must be at most %d characters", AdFormatDescriptionMaxLen),
		)
	}

	return nil
}

// adFormatImageKey names the storage object of a new image upload: unique per upload.
func adFormatImageKey(id uuid.UUID, ext string) string {
	suffix := make([]byte, adFormatKeySuffixBytes)
	_, _ = rand.Read(suffix) // crypto/rand.Read never fails; it aborts the program instead.

	return adFormatKeyPrefix + id.String() + "-" + hex.EncodeToString(suffix) + "." + ext
}
