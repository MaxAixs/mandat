// Package errs defines the application's sentinel errors.
// Callers classify failures with errors.Is against these values.
package errs

import (
	"errors"
	"fmt"
)

var (
	ErrNewsNotFound       = errors.New("news not found")
	ErrInvalidInput       = errors.New("invalid input")
	ErrConflict           = errors.New("resource conflict")
	ErrPayloadTooLarge    = errors.New("payload too large")
	ErrServiceUnavailable = errors.New("service unavailable")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrForbidden          = errors.New("forbidden")
	ErrTooManyRequests    = errors.New("too many requests")
)

// ValidationError describes a rejected input field.
// It unwraps to ErrInvalidInput, so errors.Is keeps working for callers
// that only need the error class.
type ValidationError struct {
	Field  string
	Reason string
}

// Error implements error.
func (e *ValidationError) Error() string {
	return fmt.Sprintf("invalid %s: %s", e.Field, e.Reason)
}

// Unwrap exposes the sentinel error used for classification.
func (e *ValidationError) Unwrap() error { return ErrInvalidInput }

// NewValidationError builds a field-scoped validation error.
func NewValidationError(field, reason string) *ValidationError {
	return &ValidationError{Field: field, Reason: reason}
}
