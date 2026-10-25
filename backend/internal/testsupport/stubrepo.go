// Package testsupport provides in-memory test doubles shared by test packages.
package testsupport

import (
	"context"
	"sort"

	"github.com/google/uuid"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
)

// StubRepo is an in-memory implementation of the news repository.
type StubRepo struct {
	Items               map[uuid.UUID]*models.News
	Conflicts           map[string]bool
	CreateErr           error
	GetErr              error
	ListErr             error
	ListPublishedErr    error
	UpdateErr           error
	DeleteErr           error
	CreateCalls         int
	LastFilter          models.ListFilter
	LastPublishedFilter models.ListFilter
}

// NewStubRepo creates an empty StubRepo.
func NewStubRepo() *StubRepo {
	return &StubRepo{
		Items:     map[uuid.UUID]*models.News{},
		Conflicts: map[string]bool{},
	}
}

// Create stores the article, unless the slug is marked as conflicting.
func (s *StubRepo) Create(_ context.Context, news *models.News) error {
	s.CreateCalls++
	if s.CreateErr != nil {
		return s.CreateErr
	}
	if s.Conflicts[news.Slug] {
		return errs.ErrConflict
	}

	if news.ID == uuid.Nil {
		news.ID = uuid.New()
	}
	stored := *news
	s.Items[news.ID] = &stored

	return nil
}

// GetBySlug returns a stored article or errs.ErrNewsNotFound.
func (s *StubRepo) GetBySlug(_ context.Context, slug string) (*models.News, error) {
	if s.GetErr != nil {
		return nil, s.GetErr
	}

	for _, item := range s.Items {
		if item.Slug == slug {
			copied := *item
			return &copied, nil
		}
	}

	return nil, errs.ErrNewsNotFound
}

// GetByID returns a stored article or errs.ErrNewsNotFound.
func (s *StubRepo) GetByID(_ context.Context, id uuid.UUID) (*models.News, error) {
	if s.GetErr != nil {
		return nil, s.GetErr
	}

	item, ok := s.Items[id]
	if !ok {
		return nil, errs.ErrNewsNotFound
	}
	copied := *item

	return &copied, nil
}

// List returns a page of stored articles.
func (s *StubRepo) List(_ context.Context, filter models.ListFilter) ([]*models.News, error) {
	s.LastFilter = filter
	if s.ListErr != nil {
		return nil, s.ListErr
	}

	matched := []*models.News{}
	for _, item := range s.Items {
		if filter.Status != nil && item.Status != *filter.Status {
			continue
		}
		matched = append(matched, item)
	}
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].ID.String() > matched[j].ID.String()
	})

	offset := (filter.Page - 1) * filter.Limit
	if offset > len(matched) {
		offset = len(matched)
	}
	end := offset + filter.Limit
	if end > len(matched) {
		end = len(matched)
	}

	return matched[offset:end], nil
}

// ListPublished returns a page of published articles, newest publication first.
func (s *StubRepo) ListPublished(_ context.Context, filter models.ListFilter) ([]*models.News, error) {
	s.LastPublishedFilter = filter
	if s.ListPublishedErr != nil {
		return nil, s.ListPublishedErr
	}

	matched := []*models.News{}
	for _, item := range s.Items {
		if item.Status == models.StatusPublished {
			matched = append(matched, item)
		}
	}
	sort.Slice(matched, func(i, j int) bool {
		if matched[i].PublishedAt == nil {
			return false
		}
		if matched[j].PublishedAt == nil {
			return true
		}
		if matched[i].PublishedAt.Equal(*matched[j].PublishedAt) {
			return matched[i].ID.String() > matched[j].ID.String()
		}

		return matched[i].PublishedAt.After(*matched[j].PublishedAt)
	})

	offset := (filter.Page - 1) * filter.Limit
	if offset > len(matched) {
		offset = len(matched)
	}
	end := offset + filter.Limit
	if end > len(matched) {
		end = len(matched)
	}

	return matched[offset:end], nil
}

// Update replaces a stored article.
func (s *StubRepo) Update(_ context.Context, news *models.News) error {
	if s.UpdateErr != nil {
		return s.UpdateErr
	}
	if _, ok := s.Items[news.ID]; !ok {
		return errs.ErrNewsNotFound
	}

	stored := *news
	s.Items[news.ID] = &stored

	return nil
}

// Delete removes a stored article.
func (s *StubRepo) Delete(_ context.Context, id uuid.UUID) error {
	if s.DeleteErr != nil {
		return s.DeleteErr
	}
	if _, ok := s.Items[id]; !ok {
		return errs.ErrNewsNotFound
	}

	delete(s.Items, id)

	return nil
}
