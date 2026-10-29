package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/maxaixs/mandat/backend/internal/errs"
)

const viewCooldown = time.Hour

// ViewRepository is the persistence contract used by ViewService.
type ViewRepository interface {
	RecordView(ctx context.Context, slug string, viewerID uuid.UUID, cooldown time.Duration) (int64, error)
}

// ViewService implements public article view throttling.
type ViewService struct {
	repo   ViewRepository
	logger *slog.Logger
}

// NewViewService creates a ViewService over repo.
func NewViewService(repo ViewRepository, logger *slog.Logger) *ViewService {
	return &ViewService{repo: repo, logger: logger}
}

// RecordView records at most one view per visitor per article each hour.
func (s *ViewService) RecordView(ctx context.Context, slug string, viewerID uuid.UUID) (int64, error) {
	const fc = "viewService_RecordView"

	slug = strings.TrimSpace(slug)
	if slug == "" || viewerID == uuid.Nil {
		return 0, fmt.Errorf("%s: %w", fc, errs.ErrInvalidInput)
	}

	views, err := s.repo.RecordView(ctx, slug, viewerID, viewCooldown)
	if err != nil {
		if !errors.Is(err, errs.ErrNewsNotFound) {
			s.logger.ErrorContext(ctx, "failed to record article view", "fc", fc, "error", err)
		}

		return 0, fmt.Errorf("%s: %w", fc, err)
	}

	return views, nil
}
