package service

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
)

// maxLanguageLen bounds the language code length.
const maxLanguageLen = 10

// validateNews normalizes the input in place and checks the editable article fields.
func validateNews(input *NewsInput) error {
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		return errs.NewValidationError("title", "title is required")
	}

	if input.Category == "" {
		return errs.NewValidationError("category", "category is required")
	}
	if !input.Category.Valid() {
		return errs.NewValidationError("category", fmt.Sprintf("unknown category %q", input.Category))
	}

	status := models.StatusDraft
	if input.Status != "" {
		status = input.Status
		if !status.Valid() {
			return errs.NewValidationError("status", fmt.Sprintf("unknown status %q", input.Status))
		}
	}
	if status == models.StatusPublished && strings.TrimSpace(input.Text) == "" {
		return errs.NewValidationError("text", "text is required for published articles")
	}

	input.Source = strings.TrimSpace(input.Source)
	input.SourceURL = strings.TrimSpace(input.SourceURL)
	if input.SourceURL != "" && input.Source == "" {
		return errs.NewValidationError("source", "source is required when source_url is set")
	}

	if utf8.RuneCountInString(input.Language) > maxLanguageLen {
		return errs.NewValidationError("language", "language is too long")
	}

	if slugify(input.Title) == "" {
		return errs.NewValidationError("title", "cannot derive a slug from the title")
	}

	return nil
}
