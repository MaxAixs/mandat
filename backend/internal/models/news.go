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
	CategoryNews          Category = "Новини"
	CategoryAnalytics     Category = "Аналітика"
	CategoryInsides       Category = "Інсайди"
	CategoryImportant     Category = "Важливо"
	CategoryInvestigation Category = "Розслідування"
)

// Valid reports whether c is one of the fixed rubrics.
func (c Category) Valid() bool {
	switch c {
	case CategoryNews, CategoryAnalytics, CategoryInsides,
		CategoryImportant, CategoryInvestigation:
		return true
	}
	return false
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
	ImageUrl    string
	PublishedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ListFilter selects a page of news for the admin list.
// Nil Status means all statuses. Page starts at 1.
type ListFilter struct {
	Status *Status
	Page   int
	Limit  int
}
