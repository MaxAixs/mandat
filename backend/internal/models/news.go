// Package models contains the domain types of the news platform.
package models

import (
	"time"

	"github.com/google/uuid"
)

// Status is the publication state of a news article.
type Status string

const (
	StatusDraft     Status = "draft"
	StatusPublished Status = "published"
)

// Valid reports whether s is a known status.
func (s Status) Valid() bool {
	return s == StatusDraft || s == StatusPublished
}

// Category is a content rubric; values must match the frontend union type.
type Category string

const (
	CategoryNews      Category = "Новини"
	CategoryAnalytics Category = "Міжнародні новини"
	CategoryInsides   Category = "Спорт"
	CategoryImportant Category = "Інтерв'ю"
	CategoryEconomics Category = "Економіка"
	CategoryPolitics  Category = "Політика"
	CategoryRegional  Category = "Регіональні новини"
)

// Valid reports whether c is one of the fixed rubrics.
func (c Category) Valid() bool {
	switch c {
	case CategoryNews, CategoryAnalytics, CategoryInsides,
		CategoryImportant, CategoryEconomics,
		CategoryPolitics, CategoryRegional:
		return true
	}
	return false
}

// VideoKind tells how an article video is delivered.
type VideoKind string

const (
	// VideoKindFile is a file stored in our object storage.
	VideoKindFile VideoKind = "file"
	// VideoKindEmbed is a canonical YouTube or Vimeo player URL.
	VideoKindEmbed VideoKind = "embed"
)

// Valid reports whether k is a known video kind.
func (k VideoKind) Valid() bool {
	return k == VideoKindFile || k == VideoKindEmbed
}

// Video is the single video attached to an article.
type Video struct {
	Kind VideoKind
	URL  string
}

// News is a news article. Views and timestamps are system-owned:
// clients never set them through the API.
type News struct {
	ID          uuid.UUID
	Slug        string
	Category    Category
	Title       string
	Description string
	Text        string
	Language    string
	Status      Status
	Views       int64
	Featured    bool
	MainEvent   bool
	ImageURL    string
	Source      string
	SourceURL   string
	// Video is nil when the article has no video.
	Video *Video
	// ImageURLs are the extra images in display order; empty means none.
	ImageURLs   []string
	PublishedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ListFilter selects a window of news: up to Limit rows after skipping Offset.
// Nil Status and Category mean all statuses and categories.
type ListFilter struct {
	Status   *Status
	Category *Category
	Limit    int
	Offset   int
}
