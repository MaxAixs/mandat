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

// ValidationErr describes a rejected input field.
// It unwraps to errs.ErrInvalidInput, so errors.Is keeps working for callers
// that only need the error class.
type ValidationErr struct {
	Field  string
	Reason string
}

// Error implements error.
func (e *ValidationErr) Error() string {
	return fmt.Sprintf("invalid %s: %s", e.Field, e.Reason)
}

// Unwrap exposes the sentinel error used for classification.
func (e *ValidationErr) Unwrap() error { return errs.ErrInvalidInput }

// validateNews normalizes and checks the editable article input.
// It fills a generated slug and trims the title in place.
func validateNews(input *NewsInput) error {
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		return validationErr("title", "title is required")
	}

	category := models.Category(input.Category)
	if category == "" {
		return validationErr("category", "category is required")
	}
	if !category.Valid() {
		return validationErr("category", fmt.Sprintf("unknown category %q", input.Category))
	}

	status := models.StatusDraft
	if input.Status != "" {
		status = models.Status(input.Status)
		if !status.Valid() {
			return validationErr("status", fmt.Sprintf("unknown status %q", input.Status))
		}
	}
	if status == models.StatusPublished && strings.TrimSpace(input.Text) == "" {
		return validationErr("text", "text is required for published articles")
	}

	if utf8.RuneCountInString(input.Language) > maxLanguageLen {
		return validationErr("language", "language is too long")
	}

	if slugify(input.Title) == "" {
		return validationErr("title", "cannot derive a slug from the title")
	}

	return nil
}

// validationErr builds a field-scoped validation error.
func validationErr(field, reason string) error {
	return &ValidationErr{Field: field, Reason: reason}
}
